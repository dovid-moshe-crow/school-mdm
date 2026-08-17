from PIL import Image
from pathlib import Path

W, H = 2048, 2732
src = Image.open(r"C:\Users\dovid\Downloads\photo_2026-08-12_19-13-21.jpg")
bg = Image.new("RGB", (W, H), (11, 61, 46))
scale = min(W / src.width, H / src.height)
nw, nh = int(src.width * scale), int(src.height * scale)
r = src.resize((nw, nh), Image.Resampling.LANCZOS).convert("RGB")
bg.paste(r, ((W - nw) // 2, (H - nh) // 2))
out = Path(r"C:\Users\dovid\Desktop\github\school-mdm\mobile\asc-screenshots\ipad-13-2048x2732.png")
out.parent.mkdir(parents=True, exist_ok=True)
bg.save(out, "PNG")
print("wrote", out, bg.size)
