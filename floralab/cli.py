"""FloraLab CLI application using Typer."""

import os
import subprocess
import time
from pathlib import Path
from typing import Optional

import httpx
import typer

app = typer.Typer(
    name="floralab",
    help="FloraLab CLI - Manage Flower-AI federated learning on SLURM clusters",
)


def get_florago_binary_path() -> Path:
    """Get the path to the bundled florago binary."""
    # Binary is bundled in the package
    pkg_dir = Path(__file__).parent
    binary_path = pkg_dir / "bin" / "florago-amd64"

    if not binary_path.exists():
        typer.secho(f"✗ florago binary not found at: {binary_path}", fg=typer.colors.RED)
        typer.echo("  The florago binary should be bundled with the floralab package.")
        raise typer.Exit(1)

    return binary_path


def read_pyproject_toml(project_dir: Path) -> dict:
    """Read and parse pyproject.toml using tomllib (Python 3.11+)."""
    import tomllib

    pyproject_path = project_dir / "pyproject.toml"
    if not pyproject_path.exists():
        typer.secho(f"✗ pyproject.toml not found in {project_dir}", fg=typer.colors.RED)
        raise typer.Exit(1)

    with open(pyproject_path, "rb") as f:
        return tomllib.load(f)


def write_pyproject_toml(project_dir: Path, data: dict) -> None:
    """Write data to pyproject.toml."""
    import tomli_w

    pyproject_path = project_dir / "pyproject.toml"
    with open(pyproject_path, "wb") as f:
        tomli_w.dump(data, f)


def get_api_url() -> str:
    """Get the florago API server URL from environment or default."""
    return os.getenv("FLORAGO_API_URL", "http://localhost:8080")


@app.command()
def ui(
    api_url: Optional[str] = typer.Option(None, "--api-url", help="Override florago API URL"),
    port: int = typer.Option(3000, "--port", "-p", help="Port to run the dashboard on"),
    host: str = typer.Option("127.0.0.1", "--host", help="Host to bind the dashboard to"),
) -> None:
    """Launch the FloraLab web dashboard.

    Opens a minimal web interface to monitor Flower stack status, SLURM jobs,
    and cluster information in real-time.
    """
    import uvicorn

    from floralab.ui_server import create_app

    url = api_url or get_api_url()

    typer.echo("🌐 Starting FloraLab Dashboard...")
    typer.echo(f"   API: {url}")
    typer.echo(f"   Dashboard: http://{host}:{port}")
    typer.echo("\nPress Ctrl+C to stop the dashboard\n")

    # Create and run the FastAPI app
    app_instance = create_app(url)
    uvicorn.run(app_instance, host=host, port=port, log_level="warning")


@app.command()
def init(
    login_node: Optional[str] = typer.Argument(None, help="SLURM login node hostname"),
    project_dir: Path = typer.Option(Path.cwd(), "--dir", "-d", help="Project directory"),
) -> None:
    """Initialize floralab configuration in pyproject.toml.

    This adds [tool.floralab] and [tool.flwr.federations.floralab] sections
    to your pyproject.toml if they don't exist.

    If login_node is not provided, it will:
    1. Check existing [tool.floralab] configuration
    2. Prompt interactively if missing
    """
    typer.echo("📝 Initializing floralab configuration...")
    typer.echo(f"   Project: {project_dir}")

    # Read pyproject.toml
    try:
        config = read_pyproject_toml(project_dir)
    except Exception as e:
        typer.secho(f"✗ Failed to read pyproject.toml: {e}", fg=typer.colors.RED)
        raise typer.Exit(1)

    # Check if floralab config exists
    if "tool" not in config:
        config["tool"] = {}

    modified = False

    # Determine login_node value
    existing_login_node = None
    if "floralab" in config["tool"]:
        existing_login_node = config["tool"]["floralab"].get("login-node")

    # If login_node not provided as argument
    if not login_node:
        if existing_login_node:
            # Use existing value
            login_node = existing_login_node
            typer.echo(f"   Using existing login node: {login_node}")
        else:
            # Use placeholder default value
            login_node = "slurm-login-node.example.com"
            typer.echo(f"   Using default login node: {login_node}")
            typer.secho("\n   ⚠️  Please update the login node in pyproject.toml:", fg=typer.colors.YELLOW)
            typer.echo("      [tool.floralab]")
            typer.echo(f'      login-node = "{login_node}"  # Change this to your SLURM cluster')
    else:
        typer.echo(f"   Login node: {login_node}")

    # Add or update [tool.floralab]
    if "floralab" not in config["tool"]:
        config["tool"]["floralab"] = {
            "login-node": login_node,
        }
        typer.echo("✓ Added [tool.floralab] configuration")
        modified = True
    else:
        typer.echo("  [tool.floralab] already exists")
        # Update login-node if different
        if config["tool"]["floralab"].get("login-node") != login_node:
            config["tool"]["floralab"]["login-node"] = login_node
            typer.echo(f"  Updated login-node to: {login_node}")
            modified = True

    # Add [tool.flwr.federations.floralab]
    if "flwr" not in config["tool"]:
        config["tool"]["flwr"] = {}
    if "federations" not in config["tool"]["flwr"]:
        config["tool"]["flwr"]["federations"] = {}

    if "floralab" not in config["tool"]["flwr"]["federations"]:
        config["tool"]["flwr"]["federations"]["floralab"] = {
            "address": "127.0.0.1:9093",  # Default, will be updated by run command
            "insecure": True,
        }
        typer.echo("✓ Added [tool.flwr.federations.floralab] configuration")
        modified = True
    else:
        typer.echo("  [tool.flwr.federations.floralab] already exists")

    # Write back if modified
    if modified:
        try:
            write_pyproject_toml(project_dir, config)
            typer.secho("\n✨ Configuration initialized successfully!", fg=typer.colors.GREEN)
        except Exception as e:
            typer.secho(f"✗ Failed to write pyproject.toml: {e}", fg=typer.colors.RED)
            raise typer.Exit(1)
    else:
        typer.echo("\n  Configuration already up to date")


@app.command()
def run(
    num_nodes: int = typer.Option(2, "--nodes", "-n", help="Number of client nodes"),
    partition: Optional[str] = typer.Option(None, "--partition", "-p", help="SLURM partition"),
    memory: Optional[str] = typer.Option(None, "--memory", "-m", help="Memory per node"),
    time_limit: Optional[str] = typer.Option(None, "--time", "-t", help="Time limit"),
    project_dir: Path = typer.Option(Path.cwd(), "--dir", "-d", help="Project directory"),
    ssh_port: int = typer.Option(8080, "--ssh-port", help="Local port for SSH tunnel"),
) -> None:
    """Run a Flower federated learning job on SLURM cluster.

    This command:
    1. Copies florago binary to the SLURM login node
    2. Initializes florago environment
    3. Starts florago API server
    4. Creates SSH tunnel
    5. Spins up Flower stack
    6. Updates pyproject.toml with server address
    7. Runs 'flwr run floralab .'
    """
    typer.echo("🚀 Starting Flower federated learning job...")

    # Read configuration
    try:
        config = read_pyproject_toml(project_dir)
    except Exception as e:
        typer.secho(f"✗ Failed to read pyproject.toml: {e}", fg=typer.colors.RED)
        typer.echo("  Run 'floralab-cli init <login-node>' first")
        raise typer.Exit(1)

    # Get login node
    if "tool" not in config or "floralab" not in config["tool"]:
        typer.secho("✗ floralab configuration not found in pyproject.toml", fg=typer.colors.RED)
        typer.echo("\n  Initialize configuration with:")
        typer.echo("    floralab-cli init")
        typer.echo("\n  Or specify login node directly:")
        typer.echo("    floralab-cli init <login-node>")
        raise typer.Exit(1)

    login_node = config["tool"]["floralab"].get("login-node")
    if not login_node:
        typer.secho("✗ login-node not configured in [tool.floralab]", fg=typer.colors.RED)
        typer.echo("\n  Run: floralab-cli init <login-node>")
        raise typer.Exit(1)

    typer.echo(f"   Login node: {login_node}")
    typer.echo(f"   Client nodes: {num_nodes}")

    # Step 1: Copy florago binary to remote
    typer.echo("\n📦 Step 1/8: Copying florago binary to SLURM login node...")
    florago_binary = get_florago_binary_path()
    typer.echo(f"   Local binary: {florago_binary}")
    typer.echo(f"   Remote target: {login_node}:~/florago-amd64")

    try:
        result = subprocess.run(
            ["scp", str(florago_binary), f"{login_node}:~/florago-amd64"],
            capture_output=True,
            text=True,
            check=True,
        )
        typer.echo("✓ florago binary copied successfully")
        if result.stdout:
            typer.echo(f"   stdout: {result.stdout.strip()}")
    except subprocess.CalledProcessError as e:
        typer.secho(f"✗ Failed to copy binary: {e.stderr}", fg=typer.colors.RED)
        if e.stdout:
            typer.echo(f"   stdout: {e.stdout}")
        raise typer.Exit(1)

    # Make it executable
    try:
        subprocess.run(
            ["ssh", login_node, "chmod +x ~/florago-amd64"],
            capture_output=True,
            text=True,
            check=True,
        )
    except subprocess.CalledProcessError:
        pass  # Ignore if fails

    # Step 2: Run florago init
    typer.echo("\n🔧 Step 2/8: Initializing florago environment...")
    typer.echo("   Running: ssh {} ~/florago-amd64 init".format(login_node))
    try:
        result = subprocess.run(
            ["ssh", login_node, "~/florago-amd64 init"],
            capture_output=True,
            text=True,
            timeout=300,  # 5 minutes timeout
        )
        typer.echo(f"   Return code: {result.returncode}")
        if result.stdout:
            typer.echo("   === stdout ===")
            for line in result.stdout.strip().split("\n"):
                typer.echo(f"   {line}")
        if result.stderr:
            typer.echo("   === stderr ===")
            for line in result.stderr.strip().split("\n"):
                typer.echo(f"   {line}")

        if result.returncode != 0:
            # Check if already initialized
            if "already" not in result.stdout.lower() and "already" not in result.stderr.lower():
                typer.secho(f"✗ florago init failed with exit code {result.returncode}", fg=typer.colors.RED)
                raise typer.Exit(1)
        typer.echo("✓ Florago environment ready")
    except subprocess.TimeoutExpired:
        typer.secho("✗ florago init timed out (300s)", fg=typer.colors.RED)
        raise typer.Exit(1)
    except subprocess.CalledProcessError as e:
        typer.secho(f"✗ florago init failed: {e.stderr}", fg=typer.colors.RED)
        raise typer.Exit(1)

    # Step 3: Copy Caddy and Delve binaries
    typer.echo("\n📦 Step 3/8: Copying Caddy and Delve binaries...")
    pkg_dir = Path(__file__).parent
    caddy_binary = pkg_dir / "bin" / "caddy-amd64"
    delve_binary = pkg_dir / "bin" / "dlv-amd64"

    typer.echo(f"   Package dir: {pkg_dir}")
    typer.echo(f"   Caddy binary: {caddy_binary} (exists: {caddy_binary.exists()})")
    typer.echo(f"   Delve binary: {delve_binary} (exists: {delve_binary.exists()})")

    # Create .florago/bin directory on remote
    typer.echo("   Creating remote directory: ~/.florago/bin")
    try:
        result = subprocess.run(
            ["ssh", login_node, "mkdir -p ~/.florago/bin"],
            capture_output=True,
            text=True,
            check=True,
        )
        typer.echo("   ✓ Remote directory ready")
    except subprocess.CalledProcessError as e:
        typer.echo(f"   Directory might exist: {e.stderr}")

    # Copy Caddy
    if caddy_binary.exists():
        typer.echo(f"   Copying Caddy: {caddy_binary} -> {login_node}:~/.florago/bin/caddy")
        try:
            result = subprocess.run(
                ["scp", str(caddy_binary), f"{login_node}:~/.florago/bin/caddy"],
                capture_output=True,
                text=True,
                check=True,
            )
            subprocess.run(
                ["ssh", login_node, "chmod +x ~/.florago/bin/caddy"],
                capture_output=True,
                text=True,
                check=True,
            )
            # Verify
            verify = subprocess.run(
                ["ssh", login_node, "~/.florago/bin/caddy version"],
                capture_output=True,
                text=True,
            )
            typer.echo(
                f"✓ Caddy binary copied and verified: {verify.stdout.strip() if verify.returncode == 0 else 'verification failed'}"
            )
        except subprocess.CalledProcessError as e:
            typer.secho(f"⚠ Warning: Failed to copy Caddy: {e.stderr}", fg=typer.colors.YELLOW)
    else:
        typer.secho(f"⚠ Warning: Caddy binary not found at {caddy_binary}", fg=typer.colors.YELLOW)

    # Copy Delve
    if delve_binary.exists():
        typer.echo(f"   Copying Delve: {delve_binary} -> {login_node}:~/.florago/bin/dlv")
        try:
            result = subprocess.run(
                ["scp", str(delve_binary), f"{login_node}:~/.florago/bin/dlv"],
                capture_output=True,
                text=True,
                check=True,
            )
            subprocess.run(
                ["ssh", login_node, "chmod +x ~/.florago/bin/dlv"],
                capture_output=True,
                text=True,
                check=True,
            )
            # Verify
            verify = subprocess.run(
                ["ssh", login_node, "~/.florago/bin/dlv version"],
                capture_output=True,
                text=True,
            )
            typer.echo(
                f"✓ Delve binary copied and verified: {verify.stdout.strip() if verify.returncode == 0 else 'verification failed'}"
            )
        except subprocess.CalledProcessError as e:
            typer.secho(f"⚠ Warning: Failed to copy Delve: {e.stderr}", fg=typer.colors.YELLOW)
    else:
        typer.secho(f"⚠ Warning: Delve binary not found at {delve_binary}", fg=typer.colors.YELLOW)

    # Step 4: Start florago server (in background)
    typer.echo("\n🌐 Step 4/8: Starting florago API server...")
    typer.echo(
        "   Running: ssh {} 'nohup ~/florago-amd64 start --host 0.0.0.0 --port 8080 > ~/.florago/logs/florago-server.log 2>&1 &'".format(
            login_node
        )
    )
    try:
        # Start server in background with nohup
        result = subprocess.run(
            [
                "ssh",
                login_node,
                "nohup ~/florago-amd64 start --host 0.0.0.0 --port 8080 > ~/.florago/logs/florago-server.log 2>&1 &",
            ],
            capture_output=True,
            text=True,
            shell=False,
        )
        typer.echo(f"   Command exit code: {result.returncode}")
        if result.stdout:
            typer.echo(f"   stdout: {result.stdout.strip()}")
        if result.stderr:
            typer.echo(f"   stderr: {result.stderr.strip()}")

        time.sleep(2)  # Give it time to start

        # Check if server is running
        check = subprocess.run(
            ["ssh", login_node, "pgrep -f 'florago-amd64 start'"],
            capture_output=True,
            text=True,
        )
        if check.returncode == 0:
            typer.echo(f"   ✓ Server process running (PID: {check.stdout.strip()})")
        else:
            typer.secho("   ⚠ Warning: Could not verify server process", fg=typer.colors.YELLOW)

        typer.echo("✓ API server started")
    except Exception as e:
        typer.secho(f"✗ Failed to start API server: {e}", fg=typer.colors.RED)
        raise typer.Exit(1)

    # Step 5: Create SSH tunnel
    typer.echo(f"\n🔌 Step 5/8: Creating SSH tunnel (localhost:{ssh_port} -> {login_node}:8080)...")
    typer.echo(f"   Command: ssh -N -L {ssh_port}:localhost:8080 {login_node}")
    tunnel_process = None
    try:
        tunnel_process = subprocess.Popen(
            ["ssh", "-N", "-L", f"{ssh_port}:localhost:8080", login_node],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )
        typer.echo(f"   Tunnel process PID: {tunnel_process.pid}")
        time.sleep(2)  # Give tunnel time to establish

        # Check if tunnel is alive
        if tunnel_process.poll() is not None:
            stderr_output = tunnel_process.stderr.read().decode() if tunnel_process.stderr else ""
            typer.secho(f"✗ SSH tunnel failed to establish: {stderr_output}", fg=typer.colors.RED)
            raise typer.Exit(1)

        typer.echo("✓ SSH tunnel established")
    except Exception as e:
        typer.secho(f"✗ Failed to create SSH tunnel: {e}", fg=typer.colors.RED)
        if tunnel_process:
            tunnel_process.kill()
        raise typer.Exit(1)

    # Step 6: Spin up Flower stack via API
    typer.echo(f"\n🌸 Step 6/8: Spinning up Flower stack ({num_nodes} client nodes)...")
    api_url = f"http://localhost:{ssh_port}"
    typer.echo(f"   API URL: {api_url}")

    payload = {"num_nodes": num_nodes}
    if partition:
        payload["partition"] = partition  # type: ignore
    if memory:
        payload["memory"] = memory  # type: ignore
    if time_limit:
        payload["time_limit"] = time_limit  # type: ignore

    typer.echo(f"   Payload: {payload}")

    try:
        typer.echo(f"   POST {api_url}/api/spin...")
        response = httpx.post(f"{api_url}/api/spin", json=payload, timeout=30.0)
        typer.echo(f"   Response status: {response.status_code}")
        typer.echo(f"   Response body: {response.text}")
        response.raise_for_status()

        data = response.json()
        if not data.get("success"):
            if "already running" in data.get("message", "").lower():
                typer.secho("✗ A Flower stack is already running", fg=typer.colors.RED)
                typer.echo("  Run 'floralab-cli stop' to tear down the existing stack first")
                if tunnel_process:
                    tunnel_process.kill()
                raise typer.Exit(1)
            else:
                typer.secho(f"✗ Failed to spin up stack: {data.get('message')}", fg=typer.colors.RED)
                if tunnel_process:
                    tunnel_process.kill()
                raise typer.Exit(1)

        job_id = data.get("job_id")
        typer.echo(f"✓ Flower stack job submitted: {job_id}")

    except httpx.HTTPError as e:
        typer.secho(f"✗ API request failed: {e}", fg=typer.colors.RED)
        if tunnel_process:
            tunnel_process.kill()
        raise typer.Exit(1)

    # Step 7: Wait for stack to be ready and get server info
    typer.echo("\n⏳ Step 7/8: Waiting for Flower stack to be ready...")
    typer.echo(f"   Max wait time: {300}s")
    max_wait = 300  # 5 minutes
    start_time = time.time()
    server_ready = False
    control_port = None
    poll_count = 0

    while time.time() - start_time < max_wait:
        poll_count += 1
        elapsed = int(time.time() - start_time)
        try:
            typer.echo(f"   Poll #{poll_count} (elapsed: {elapsed}s) - GET {api_url}/api/spin")
            response = httpx.get(f"{api_url}/api/spin", timeout=10.0)
            typer.echo(f"   Response status: {response.status_code}")
            response.raise_for_status()

            data = response.json()
            state = data.get("state", {})
            typer.echo(f"   State status: {state.get('status')}")

            if state.get("status") == "running":
                server_node = state.get("server_node")
                typer.echo(f"   Server node: {server_node}")
                if server_node and server_node.get("status") == "ready":
                    control_port = server_node.get("control_api_port")
                    typer.echo(f"   Control port: {control_port}")
                    if control_port:
                        server_ready = True
                        typer.echo(f"✓ Flower stack is ready (control API: localhost:{control_port})")
                        break

            # Show progress
            completed = state.get("completed_nodes", 0)
            expected = state.get("expected_nodes", num_nodes + 1)
            typer.echo(f"  Progress: {completed}/{expected} nodes ready...")
            time.sleep(5)

        except Exception as e:
            typer.echo(f"  Waiting... (error: {e})")
            time.sleep(5)

    if not server_ready or not control_port:
        elapsed = int(time.time() - start_time)
        typer.secho(f"✗ Flower stack did not become ready in time ({elapsed}s elapsed)", fg=typer.colors.RED)
        if tunnel_process:
            tunnel_process.kill()
        raise typer.Exit(1)

    # Update pyproject.toml with control API address
    typer.echo("\n📝 Updating pyproject.toml with control API address...")
    typer.echo(f"   Setting federation address: 127.0.0.1:{control_port}")
    try:
        config["tool"]["flwr"]["federations"]["floralab"]["address"] = f"127.0.0.1:{control_port}"
        write_pyproject_toml(project_dir, config)
        typer.echo(f"✓ Updated federation address to 127.0.0.1:{control_port}")
    except Exception as e:
        typer.secho(f"⚠ Warning: Failed to update pyproject.toml: {e}", fg=typer.colors.YELLOW)

    # Step 8: Run flwr
    typer.echo("\n🎯 Step 8/8: Running Flower federated learning job...")
    typer.echo(f"   Working directory: {project_dir}")
    typer.echo("   Command: flwr run floralab .")
    typer.echo(f"   Federation: floralab @ 127.0.0.1:{control_port}")

    try:
        # Run flwr in the project directory
        typer.echo("\n   Starting flwr run...")
        result = subprocess.run(
            ["flwr", "run", "floralab", "."],
            cwd=project_dir,
            check=True,
        )
        typer.echo(f"   flwr exit code: {result.returncode}")

        typer.secho("\n✨ Federated learning job completed successfully!", fg=typer.colors.GREEN)

    except subprocess.CalledProcessError as e:
        typer.secho(f"\n✗ flwr run failed with exit code {e.returncode}", fg=typer.colors.RED)
    except KeyboardInterrupt:
        typer.echo("\n\n⚠ Interrupted by user")
    finally:
        # Cleanup: close tunnel
        if tunnel_process:
            typer.echo("\n🧹 Cleaning up SSH tunnel...")
            tunnel_process.kill()
            tunnel_process.wait()


@app.command()
def stop(
    project_dir: Path = typer.Option(Path.cwd(), "--dir", "-d", help="Project directory"),
    ssh_port: int = typer.Option(8080, "--ssh-port", help="Local port for SSH tunnel"),
) -> None:
    """Stop the running Flower stack on SLURM cluster."""
    typer.echo("🛑 Stopping Flower stack...")

    # Read configuration
    try:
        config = read_pyproject_toml(project_dir)
    except Exception as e:
        typer.secho(f"✗ Failed to read pyproject.toml: {e}", fg=typer.colors.RED)
        raise typer.Exit(1)

    # Get login node
    if "tool" not in config or "floralab" not in config["tool"]:
        typer.secho("✗ floralab configuration not found in pyproject.toml", fg=typer.colors.RED)
        typer.echo("\n  Run: floralab-cli init")
        raise typer.Exit(1)

    login_node = config["tool"]["floralab"].get("login-node")
    if not login_node:
        typer.secho("✗ login-node not configured in [tool.floralab]", fg=typer.colors.RED)
        typer.echo("\n  Run: floralab-cli init <login-node>")
        raise typer.Exit(1)

    typer.echo(f"   Login node: {login_node}")

    # Create temporary SSH tunnel
    typer.echo("   Creating temporary SSH tunnel...")
    tunnel_process = None
    try:
        tunnel_process = subprocess.Popen(
            ["ssh", "-N", "-L", f"{ssh_port}:localhost:8080", login_node],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )
        time.sleep(2)

        api_url = f"http://localhost:{ssh_port}"

        # Call DELETE /api/spin
        response = httpx.delete(f"{api_url}/api/spin", timeout=10.0)
        response.raise_for_status()

        data = response.json()
        if data.get("success"):
            typer.secho(f"✓ Flower stack stopped: {data.get('job_id')}", fg=typer.colors.GREEN)
        else:
            typer.secho(f"✗ {data.get('message')}", fg=typer.colors.RED)

    except httpx.HTTPError as e:
        typer.secho(f"✗ Failed to stop stack: {e}", fg=typer.colors.RED)
        raise typer.Exit(1)
    except Exception as e:
        typer.secho(f"✗ Error: {e}", fg=typer.colors.RED)
        raise typer.Exit(1)
    finally:
        if tunnel_process:
            tunnel_process.kill()
            tunnel_process.wait()


@app.command()
def spin(
    num_nodes: int = typer.Argument(..., help="Number of client nodes to deploy"),
    partition: Optional[str] = typer.Option(None, "--partition", "-p", help="SLURM partition"),
    memory: Optional[str] = typer.Option(None, "--memory", "-m", help="Memory per node (e.g., 4G)"),
    time_limit: Optional[str] = typer.Option(None, "--time", "-t", help="Time limit (e.g., 01:00:00)"),
    api_url: Optional[str] = typer.Option(None, "--api-url", help="Override florago API URL"),
) -> None:
    """Spin up a Flower-AI stack on SLURM cluster.

    This submits a SLURM job that deploys 1 server node + N client nodes
    running the Flower federated learning stack.
    """
    url = api_url or get_api_url()

    payload = {
        "num_nodes": num_nodes,
    }

    if partition:
        payload["partition"] = partition  # type: ignore
    if memory:
        payload["memory"] = memory  # type: ignore
    if time_limit:
        payload["time_limit"] = time_limit  # type: ignore

    typer.echo(f"🚀 Spinning up Flower stack with {num_nodes} client nodes...")
    typer.echo(f"   API: {url}")

    try:
        response = httpx.post(f"{url}/api/spin", json=payload, timeout=30.0)
        response.raise_for_status()

        data = response.json()

        if data.get("success"):
            typer.secho(f"✓ Flower stack job submitted: {data.get('job_id')}", fg=typer.colors.GREEN)
            typer.echo(f"  Message: {data.get('message')}")

            if state := data.get("state"):
                typer.echo(f"  Status: {state.get('status')}")
                typer.echo(f"  Expected nodes: {state.get('expected_nodes')}")
        else:
            typer.secho(f"✗ Failed: {data.get('message')}", fg=typer.colors.RED)
            raise typer.Exit(1)

    except httpx.HTTPError as e:
        typer.secho(f"✗ HTTP error: {e}", fg=typer.colors.RED)
        raise typer.Exit(1)
    except Exception as e:
        typer.secho(f"✗ Error: {e}", fg=typer.colors.RED)
        raise typer.Exit(1)


@app.command()
def status(
    api_url: Optional[str] = typer.Option(None, "--api-url", help="Override florago API URL"),
    verbose: bool = typer.Option(False, "--verbose", "-v", help="Show detailed information"),
) -> None:
    """Check the status of the Flower-AI stack."""
    url = api_url or get_api_url()

    typer.echo("📊 Checking Flower stack status...")
    typer.echo(f"   API: {url}")

    try:
        response = httpx.get(f"{url}/api/spin", timeout=10.0)
        response.raise_for_status()

        data = response.json()
        state = data.get("state", {})

        typer.echo(f"\n{'=' * 60}")
        typer.echo(f"Job ID: {data.get('job_id', 'N/A')}")
        typer.echo(f"Status: {state.get('status', 'unknown')}")
        typer.echo(f"Expected Nodes: {state.get('expected_nodes', 0)}")
        typer.echo(f"Completed Nodes: {state.get('completed_nodes', 0)}")

        if server := state.get("server_node"):
            typer.echo("\n🖥️  Server Node:")
            typer.echo(f"   IP: {server.get('ip')}")
            typer.echo(f"   Status: {server.get('status')}")
            if verbose:
                typer.echo(f"   Fleet API Port: {server.get('fleet_api_port')}")
                typer.echo(f"   Control API Port: {server.get('control_api_port')}")

        if clients := state.get("client_nodes"):
            typer.echo(f"\n💻 Client Nodes ({len(clients)}):")
            for node_id, client in clients.items():
                typer.echo(f"   {node_id}: {client.get('ip')} [{client.get('status')}]")

        typer.echo(f"{'=' * 60}\n")

    except httpx.HTTPError as e:
        typer.secho(f"✗ HTTP error: {e}", fg=typer.colors.RED)
        raise typer.Exit(1)
    except Exception as e:
        typer.secho(f"✗ Error: {e}", fg=typer.colors.RED)
        raise typer.Exit(1)


@app.command()
def down(
    api_url: Optional[str] = typer.Option(None, "--api-url", help="Override florago API URL"),
    force: bool = typer.Option(False, "--force", "-f", help="Skip confirmation"),
) -> None:
    """Tear down the Flower-AI stack (cancel SLURM job)."""
    url = api_url or get_api_url()

    if not force:
        confirm = typer.confirm("⚠️  This will cancel the SLURM job and stop all Flower nodes. Continue?")
        if not confirm:
            typer.echo("Cancelled.")
            raise typer.Exit(0)

    typer.echo("🛑 Tearing down Flower stack...")
    typer.echo(f"   API: {url}")

    try:
        response = httpx.delete(f"{url}/api/spin", timeout=10.0)
        response.raise_for_status()

        data = response.json()

        if data.get("success"):
            typer.secho(f"✓ Flower stack job cancelled: {data.get('job_id')}", fg=typer.colors.GREEN)
            typer.echo(f"  Message: {data.get('message')}")
        else:
            typer.secho(f"✗ Failed: {data.get('message')}", fg=typer.colors.RED)
            raise typer.Exit(1)

    except httpx.HTTPError as e:
        typer.secho(f"✗ HTTP error: {e}", fg=typer.colors.RED)
        raise typer.Exit(1)
    except Exception as e:
        typer.secho(f"✗ Error: {e}", fg=typer.colors.RED)
        raise typer.Exit(1)


@app.command()
def monitoring(
    api_url: Optional[str] = typer.Option(None, "--api-url", help="Override florago API URL"),
) -> None:
    """Get comprehensive monitoring data (Flower stack + SLURM cluster info)."""
    url = api_url or get_api_url()

    typer.echo("📈 Fetching monitoring data...")
    typer.echo(f"   API: {url}")

    try:
        response = httpx.get(f"{url}/api/monitoring", timeout=10.0)
        response.raise_for_status()

        data = response.json()

        typer.echo(f"\n{'=' * 60}")
        typer.echo(f"Timestamp: {data.get('timestamp')}")

        # Flower stack info
        if flower := data.get("flower_stack"):
            typer.echo("\n🌸 Flower Stack:")
            typer.echo(f"   Status: {flower.get('status')}")
            typer.echo(f"   Job ID: {flower.get('job_id', 'N/A')}")
            typer.echo(f"   Nodes: {flower.get('completed_nodes')}/{flower.get('expected_nodes')}")

        # SLURM info
        if slurm := data.get("slurm_info"):
            typer.echo("\n⚡ SLURM Cluster:")
            if user := slurm.get("user"):
                typer.echo(f"   User: {user}")

            if jobs := slurm.get("jobs"):
                typer.echo("\n   Jobs:")
                typer.echo(f"   {jobs}")

            if nodes := slurm.get("nodes"):
                typer.echo("\n   Nodes:")
                typer.echo(f"   {nodes}")

        typer.echo(f"{'=' * 60}\n")

    except httpx.HTTPError as e:
        typer.secho(f"✗ HTTP error: {e}", fg=typer.colors.RED)
        raise typer.Exit(1)
    except Exception as e:
        typer.secho(f"✗ Error: {e}", fg=typer.colors.RED)
        raise typer.Exit(1)


@app.command()
def health(
    api_url: Optional[str] = typer.Option(None, "--api-url", help="Override florago API URL"),
) -> None:
    """Check if the florago API server is healthy."""
    url = api_url or get_api_url()

    try:
        response = httpx.get(f"{url}/health", timeout=5.0)
        response.raise_for_status()

        data = response.json()
        typer.secho("✓ API server is healthy", fg=typer.colors.GREEN)
        typer.echo(f"  Status: {data.get('status')}")
        typer.echo(f"  Timestamp: {data.get('timestamp')}")

    except httpx.HTTPError as e:
        typer.secho(f"✗ API server unreachable: {e}", fg=typer.colors.RED)
        raise typer.Exit(1)
    except Exception as e:
        typer.secho(f"✗ Error: {e}", fg=typer.colors.RED)
        raise typer.Exit(1)


def main():
    """Main entry point for the CLI."""
    app()


if __name__ == "__main__":
    main()
