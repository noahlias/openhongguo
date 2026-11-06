# dmgbuild settings, run by bundle-macos.sh: the app on the left, a link to
# /Applications on the right, over dmg-background.svg.
import os.path

app = defines["app"]
format = "UDZO"
files = [app]
symlinks = {"Applications": "/Applications"}
icon = defines["icon"]
background = defines["background"]
window_rect = ((200, 120), (600, 380))
icon_size = 112
text_size = 13
icon_locations = {os.path.basename(app): (160, 180), "Applications": (440, 180)}
