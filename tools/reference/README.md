# Reference lab

Tools that turn an extracted Diablo II Resurrected install into **private study material**:
viewable images, measurements and side-by-side renders. Nothing produced here is meant to ship
in another project. The goal is to learn the style (numbers, proportions, timing, lighting)
and describe it in neutral terms, then make original content from those descriptions.

Keep the outputs in `extract/` (gitignored). Do not copy them into other repositories and do not
feed them to generators as image inputs.

## Pipeline

```
casc -x -o extract/d2r -convert png         1. extract everything, textures as PNG (and DDS)
d2rconvert -in extract/d2r -out extract/reference     2. 2D sprites, tiles, material channels, 3D models (.glb)
d2ranalyze -in extract/d2r -out extract/reference/analysis   3. measurements + report.md
blender -b -P blender/build_blend.py ...    4. (3D) assemble multi-part units (characters) into a .blend
blender -b -P blender/render_sheets.py ...  5. (3D) render through the target game's camera and lights
```

Build the Go tools from the repository root:

```bash
go build -o extract/d2rconvert.exe ./tools/reference/d2rconvert
go build -o extract/d2ranalyze.exe ./tools/reference/d2ranalyze
```

## d2rconvert: original 2D assets

| Output | Source | What it is good for |
| --- | --- | --- |
| `sprites/<chars\|monsters\|objects>/<unit>/<mode>_<weapon>.png` | `.cof` + `.dcc` layers | Every animation of every unit, layers composited, one row per direction, one column per frame, anchored on the feet. The original pre-rendered look at its native size. |
| `sprites/.../<mode>_<weapon>.json` | `.cof` | Frames, directions, hit frame, speed, cell size, anchor, figure height per direction. |
| `dc6/...png` | `.dc6` | UI panels, item and inventory graphics, fonts. |
| `tiles/<act>/<set>/<name>.png` (+ `.json`) | `.dt1` | Floor, wall, roof and shadow tiles of every level set, with the act palette. |
| `orm/<path>_{ao,roughness,metal}.png` | HD `_orm` textures | Occlusion, roughness and metalness as separate greyscale images. |
| `models/<path>.glb` | HD `.model` (Granny 3D) | Meshes in metres with normals, UVs and PBR materials bound to their exact textures (albedo, normal map, occlusion/roughness/metalness). |

| `hdsprites/<path>.png` (+ `.json`) | HD `.sprite` (SpA1) | Remastered 2D art: 444 item icons, skill icons, panels, portraits, loading screens, automap. Each file is a horizontal strip of frames; the `.json` gives `frames`, `frameWidth` and `stride` (frame `i` starts at `i * stride`). |

Which art set each output comes from: `models`, `orm`, `hdsprites` and the palette/material
measurements use the remastered (HD) assets. `sprites`, `dc6`, `tiles` and the unit heights in
`proportions.csv` come from the original 2D assets kept for the legacy graphics mode.
`animation.csv` is game timing, shared by both modes.

Options: `-only sprites,dc6,tiles,orm,models,hdsprites`, `-pattern monsters/` (substring of the input path), `-j 16`.

HD sprites come in two copies, full resolution and `.lowend`; only the first is converted unless `-lowend` is passed.

Model options:
- `-lod 0` (default) converts the most detailed level only; `-lod all` converts every level.
- `-textures link` (default) references the PNGs written by `casc -convert png` with relative
  paths, so keep `extract/d2r` and `extract/reference` side by side. `-textures embed` writes
  self-contained `.glb` files (about 10-30 MB each), `-textures none` skips them.
- `-skin` also exports the skeleton and skinning weights. Experimental: the meshes are correct
  in their bind pose without it, but on some multi-part characters the exported rest pose does
  not match the inverse bind matrices and parts end up rotated.

Modes in file names: `nu` neutral, `wl` walk, `rn` run, `a1`/`a2` attacks, `s1`-`s4` skills,
`sc` cast, `gh` get hit, `dt` death, `dd` dead, `bl` block, `tn`/`tw` town idle/walk.
Weapon classes: `hth` unarmed, `1hs` one-handed swing, `2hs` two-handed sword, `bow`, `xbw`, `stf`...
For characters, the "lit" (light armour) layers are used.

The 6 `.dt1` files with a non-standard header and a few empty `.cof` files are reported as failures and skipped.

## d2ranalyze: measurements

Writes to `-out`:

| File | Content |
| --- | --- |
| `report.md` | All tables below, ready to read. |
| `palettes.json`, `palettes/<group>.png` | Per group (environment act/biome, playable classes, enemies, NPCs, objects, items, and per unit): albedo value and saturation percentiles, hue distribution, 8 dominant colours with their share (swatch image). |
| `materials.json` | Roughness / metalness / occlusion percentiles and metal share per group. |
| `proportions.csv` | Height of every unit in its neutral pose, in pixels and relative to the playable classes (needs the `sprites` output of d2rconvert). |
| `animation.csv`, `animation_modes.json` | From `animdata.d2`: frames, playback rate, duration and hit frame of every animation, and medians per mode (walk, attack, death...). |

## Blender: 3D reference renders

`blender/rig.json` holds the camera and lights: orthographic side view raised by `elevation_deg`,
models turned by `yaw_deg` toward the camera, warm key light, cold rim light, dark blue ambient,
`pixels_per_unit` for the sprite scale. Edit it to match the target game's rig.

```bash
# Assemble meshes + textures into a .blend (textures matched by name: <part>_alb/_nrm/_orm.png)
blender -b -P tools/reference/blender/build_blend.py -- --models <meshes> --textures <unit>/textures --out unit.blend

# Render every model in a folder through the rig, with flat black silhouettes
blender -b -P tools/reference/blender/render_sheets.py -- --input <models or .blend> --out <folder> --silhouette
```

Accepted inputs: `.glb`, `.gltf`, `.fbx`, `.obj`, `.blend` (and `.dae` with Blender 4.x, which
still has the Collada importer). Each render writes a `.json` with the model size in world units
and in pixels, to compare against the target game's sprite sizes.

### HD models

D2R's `.model`, `.skeleton` and `.animations` files are Granny 3D files (format 7, 64-bit),
compressed with BitKnit2. The `granny` package reads them without any external DLL (the
BitKnit2 decoder is a Go port of the MIT-licensed decoder from pybg3), and `d2rconvert -only
models` exports the meshes to glTF. Animation curves are not converted yet: use the 2D sprite
sheets and `animation.csv` for timing.

Monsters and objects are one `.model` per part or prop and can be rendered directly. Playable
classes are split in parts with armour variants (`lit`, `med`, `hvy`): assemble one variant first.

```bash
blender -b -P tools/reference/blender/build_blend.py -- --models extract/reference/models/character/player/<class> --include "head|_med_" --out <class>_med.blend
blender -b -P tools/reference/blender/render_sheets.py -- --input <class>_med.blend --out renders --silhouette
blender -b -P tools/reference/blender/render_sheets.py -- --input extract/reference/models/character/enemy --pattern lod0 --out renders/enemies
```

Rendering uses the CPU by default (`"device"` in `rig.json`, or `REF_DEVICE=auto` to use the GPU):
Cycles' AMD HIP backend crashed mid-render on large textured models during testing, leaving empty tiles.

## From measurements to original content

1. Read `report.md` and the sheets. Write the style guide of the target game from them, in
   neutral words: value and saturation ranges, hue budget, material roughness, size classes
   relative to the hero, frame counts and hit timing per action, light colours.
2. Never name the reference, its characters, places or items in the target project.
3. Generate concepts from text built on that style guide; never use reference images as
   generator inputs (image-to-image, style or IP adapters).
4. Before accepting a concept, compare it with the closest reference privately and change
   anything that reads as a copy: silhouette, palette, costume parts, pose.
