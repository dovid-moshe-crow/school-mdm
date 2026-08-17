"""Resize iPhone ASC screenshots to 6.5" accepted size 1284x2778."""

from pathlib import Path

from PIL import Image

SRC = Path(__file__).resolve().parents[1] / "mobile" / "asc-screenshots"
OUT = SRC / "upload"
TW, TH = 1284, 2778

IPHONE = [
    ("01-store-iphone.png", "iphone65-01-store-1284x2778.png"),
    ("02-request-iphone.png", "iphone65-02-request-1284x2778.png"),
    ("03-updates-iphone.png", "iphone65-03-updates-1284x2778.png"),
]

IPAD = [
    ("01-store-ipad.png", "ipad-01-store-2048x2732.png"),
    ("02-request-ipad.png", "ipad-02-request-2048x2732.png"),
    ("03-updates-ipad.png", "ipad-03-updates-2048x2732.png"),
]


def fit_cover(im: Image.Image, tw: int, th: int) -> Image.Image:
    im = im.convert("RGB")
    sw, sh = im.size
    scale = max(tw / sw, th / sh)
    nw, nh = int(round(sw * scale)), int(round(sh * scale))
    im = im.resize((nw, nh), Image.Resampling.LANCZOS)
    left = (nw - tw) // 2
    top = (nh - th) // 2
    return im.crop((left, top, left + tw, top + th))


def main() -> None:
    OUT.mkdir(parents=True, exist_ok=True)
    for old in OUT.glob("iphone-*-1290x2796.png"):
        old.unlink()
        print("removed", old.name)

    for src_name, dest_name in IPHONE:
        out = fit_cover(Image.open(SRC / src_name), TW, TH)
        dest = OUT / dest_name
        out.save(dest, "PNG", optimize=True)
        print(dest.name, out.size)

    for src_name, dest_name in IPAD:
        dest = OUT / dest_name
        Image.open(SRC / src_name).convert("RGB").save(dest, "PNG", optimize=True)
        print(dest.name, Image.open(dest).size)


if __name__ == "__main__":
    main()
