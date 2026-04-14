"""
Helper utilities for the Docksmith sample app.
"""

def banner(title: str, width: int = 50) -> str:
    """Return a simple banner string."""
    bar = "=" * width
    pad = (width - len(title) - 2) // 2
    return f"{bar}\n{' ' * pad} {title}\n{bar}"
