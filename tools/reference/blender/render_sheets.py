"""Renders models through a side-view game camera and light rig, for side-by-side reference.

blender -b -P render_sheets.py -- --input <model or folder> --out <folder> [--rig rig.json]
                                 [--silhouette] [--pattern text]

For every model (.glb, .gltf, .fbx, .obj, .blend; .dae with Blender 4.x) and every yaw in the
rig, writes <out>/<name>_yaw<deg>.png (lit, transparent background) and, with --silhouette,
<name>_yaw<deg>_silhouette.png (flat black shape, for the silhouette readability check).
A <name>.json file records the size of the model in world units and in pixels.
"""
import argparse
import json
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import common  # noqa: E402

import bpy  # noqa: E402

MODEL_EXTS = {".glb", ".gltf", ".fbx", ".obj", ".blend", ".dae"}


def find_models(path, pattern):
    if os.path.isfile(path):
        return [path]
    found = []
    for root, _, files in os.walk(path):
        for f in files:
            if os.path.splitext(f)[1].lower() in MODEL_EXTS and pattern in os.path.join(root, f):
                found.append(os.path.join(root, f))
    return sorted(found)


def render_model(path, out_dir, rig, silhouette):
    name = os.path.splitext(os.path.basename(path))[0]
    info = {"source": os.path.basename(path), "renders": []}
    for yaw in rig.get("yaw_deg", [0.0]):
        scene = common.reset_scene()
        objects = common.import_model(path)
        if not any(o.type == 'MESH' for o in objects):
            print(f"skip {path}: no mesh")
            return None
        common.rotate_yaw(objects, yaw)
        lo, hi = common.ground(objects)
        common.light_rig(scene, rig)
        common.side_camera(scene, rig, lo, hi)
        common.render_settings(scene, rig)
        base = os.path.join(out_dir, f"{name}_yaw{int(round(yaw))}")
        scene.render.filepath = base + ".png"
        bpy.ops.render.render(write_still=True)
        if silhouette:
            mat = common.silhouette_material()
            for o in objects:
                if o.type == 'MESH':
                    o.data.materials.clear()
                    o.data.materials.append(mat)
            scene.world = None
            for o in list(scene.objects):
                if o.type == 'LIGHT':
                    bpy.data.objects.remove(o)
            scene.render.filepath = base + "_silhouette.png"
            bpy.ops.render.render(write_still=True)
        size = hi - lo
        info["size_units"] = [round(size.x, 3), round(size.y, 3), round(size.z, 3)]
        info["renders"].append({"yaw": yaw, "file": os.path.basename(base) + ".png",
                                "pixels": [scene.render.resolution_x, scene.render.resolution_y],
                                "height_px": round(size.z * rig["pixels_per_unit"])})
    with open(os.path.join(out_dir, name + ".json"), "w", encoding="utf-8") as f:
        json.dump(info, f, indent=2)
    return info


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--input", required=True)
    p.add_argument("--out", required=True)
    p.add_argument("--rig", default=os.path.join(os.path.dirname(os.path.abspath(__file__)), "rig.json"))
    p.add_argument("--silhouette", action="store_true")
    p.add_argument("--pattern", default="")
    args = p.parse_args(common.script_args())
    rig = common.load_rig(args.rig)
    os.makedirs(args.out, exist_ok=True)
    models = find_models(args.input, args.pattern)
    print(f"{len(models)} models")
    for i, m in enumerate(models):
        print(f"[{i + 1}/{len(models)}] {m}")
        try:
            render_model(m, args.out, rig, args.silhouette)
        except Exception as e:  # keep going on broken files
            print(f"failed {m}: {e}")


main()
