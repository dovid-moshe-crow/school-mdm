"""Capture App Store screenshots of the KFilter portal via Chromium.

Sizes:
  - iPhone 6.7": 1290x2796
  - iPad 13":    2048x2732
"""

from __future__ import annotations

import asyncio
from pathlib import Path

from playwright.async_api import async_playwright

OUT = Path(__file__).resolve().parents[1] / "mobile" / "asc-screenshots"
DEVICE_ID = "00008140-00092C1E3CD1801C"
BASE = "https://nanok.kfilter.net"

VIEWPORTS = {
    "iphone": {"width": 430, "height": 932, "device_scale_factor": 3},  # → 1290x2796
    "ipad": {"width": 1024, "height": 1366, "device_scale_factor": 2},  # → 2048x2732
}

SHOTS = [
    ("01-store", f"/d/{DEVICE_ID}/store?client=kfilter"),
    ("02-request", f"/d/{DEVICE_ID}?tab=request&client=kfilter"),
    ("03-updates", f"/d/{DEVICE_ID}?tab=updates&client=kfilter"),
]


async def capture_one(browser, kind: str, vp: dict, name: str, path: str) -> Path:
    context = await browser.new_context(
        viewport={"width": vp["width"], "height": vp["height"]},
        device_scale_factor=vp["device_scale_factor"],
        locale="he-IL",
        color_scheme="light",
        user_agent=(
            "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) "
            "AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1"
            if kind == "iphone"
            else "Mozilla/5.0 (iPad; CPU OS 18_0 like Mac OS X) "
            "AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1"
        ),
    )
    page = await context.new_page()
    url = BASE + path
    await page.goto(url, wait_until="networkidle", timeout=60_000)
    # Native shell chrome (matches companion app bar)
    await page.evaluate(
        """() => {
          if (document.getElementById('kfilter-shell')) return;
          const bar = document.createElement('div');
          bar.id = 'kfilter-shell';
          bar.dir = 'rtl';
          bar.style.cssText = 'position:sticky;top:0;z-index:9999;background:#0b3d2e;color:#fff;'
            + 'padding:14px 16px;font:600 17px Rubik,Segoe UI,sans-serif;text-align:center;'
            + 'box-shadow:0 1px 0 rgba(0,0,0,.08)';
          bar.textContent = 'KFilter';
          document.body.prepend(bar);
          document.documentElement.style.background = '#eef5f1';
        }"""
    )
    await page.wait_for_timeout(800)
    OUT.mkdir(parents=True, exist_ok=True)
    dest = OUT / f"{name}-{kind}.png"
    await page.screenshot(path=str(dest), full_page=False, type="png")
    await context.close()
    print(f"wrote {dest}")
    return dest


async def main() -> None:
    async with async_playwright() as p:
        browser = await p.chromium.launch(headless=True)
        for kind, vp in VIEWPORTS.items():
            for name, path in SHOTS:
                await capture_one(browser, kind, vp, name, path)
        await browser.close()
    print(f"done → {OUT}")


if __name__ == "__main__":
    asyncio.run(main())
