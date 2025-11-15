"""FastAPI server for FloraLab UI dashboard."""

from pathlib import Path

import httpx
from fastapi import FastAPI, HTTPException
from fastapi.responses import HTMLResponse


def create_app(api_url: str) -> FastAPI:
    """Create FastAPI application for the dashboard."""
    app = FastAPI(title="FloraLab Dashboard")

    # Get the HTML template path
    template_path = Path(__file__).parent / "templates" / "dashboard.html"

    @app.get("/", response_class=HTMLResponse)
    async def get_dashboard():
        """Serve the dashboard HTML."""
        if not template_path.exists():
            raise HTTPException(status_code=500, detail="Dashboard template not found")

        return template_path.read_text()

    @app.get("/api/health")
    async def get_health():
        """Proxy health check from florago API."""
        try:
            async with httpx.AsyncClient() as client:
                response = await client.get(f"{api_url}/health", timeout=5.0)
                response.raise_for_status()
                return response.json()
        except Exception as e:
            raise HTTPException(status_code=503, detail=f"API unreachable: {str(e)}")

    @app.get("/api/monitoring")
    async def get_monitoring():
        """Proxy monitoring data from florago API."""
        try:
            async with httpx.AsyncClient() as client:
                response = await client.get(f"{api_url}/api/monitoring", timeout=10.0)
                response.raise_for_status()
                return response.json()
        except Exception as e:
            raise HTTPException(status_code=503, detail=f"API unreachable: {str(e)}")

    return app
