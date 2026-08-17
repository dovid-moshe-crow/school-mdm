from PIL import Image
from pathlib import Path

src = Path(r"C:\Users\dovid\Downloads\photo_2026-08-12_19-05-43.jpg")
im = Image.open(src)
print("orig", im.size, im.mode)
W, H = 1284, 2778

# Cover-fit: scale to fill, center-crop
scale = max(W / im.width, H / im.height)
nw, nh = int(im.width * scale), int(im.height * scale)
resized = im.resize((nw, nh), Image.Resampling.LANCZOS)
left = (nw - W) // 2
top = (nh - H) // 2
cropped = resized.crop((left, top, left + W, top + H))

out_dir = Path(r"C:\Users\dovid\Desktop\github\school-mdm\mobile\asc-screenshots")
out_dir.mkdir(parents=True, exist_ok=True)
out = out_dir / "real-screenshot-1284x2778.png"
cropped.convert("RGB").save(out, "PNG")

# Letterbox version (full image visible, green bars)
bg = Image.new("RGB", (W, H), (11, 61, 46))
scale2 = min(W / im.width, H / im.height)
nw2, nh2 = int(im.width * scale2), int(im.height * scale2)
r2 = im.resize((nw2, nh2), Image.Resampling.LANCZOS)
bg.paste(r2.convert("RGB"), ((W - nw2) // 2, (H - nh2) // 2))
out2 = out_dir / "real-screenshot-letterbox-1284x2778.png"
bg.save(out2, "PNG")

print("wrote", out, cropped.size)
print("wrote", out2, bg.size)
