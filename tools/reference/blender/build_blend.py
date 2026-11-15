"""Imports converted unit meshes and wires their textures, then saves a .blend.

blender -b -P build_blend.py -- --models <folder of .glb/.gltf/.dae/.fbx> --out unit.blend
                                [--include "head|_med_"] [--textures <folder of PNG>] [--lod 0]

Models converted by d2rconvert already carry their materials and textures; they are kept.
Meshes without image textures fall back to name matching in --textures: for a mesh or material
called "torso" (or "torso_lod0"), the script looks for torso_alb.png, torso_nrm.png and
torso_orm.png (or <unit>_torso_*.png). Normal maps are expected as regular RGB normal maps
(casc -convert png rebuilds them). Occlusion/roughness/metalness use the R/G/B channels of _orm.

Playable classes come in parts with armour variants (lit, med, hvy): use --include to pick one,
i.e. --include "head|_med_".
"""
import argparse
import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import common  # noqa: E402

import bpy  # noqa: E402


def index_textures(folder):
    """Maps lowercase base names (without the channel suffix) to {channel: path}."""
    table = {}
    for root, _, files in os.walk(folder):
        for f in files:
            m = re.match(r"(.+?)_(alb|nrm|orm)(\$\w+)?\.png$", f.lower())
            if m:
                table.setdefault(m.group(1), {})[m.group(2)] = os.path.join(root, f)
    return table


def find_set(table, *names):
    for n in names:
        n = re.sub(r"(_lod\d+|\.\d+)$", "", n.lower())
        if n in table:
            return table[n]
        for key, value in table.items():
            if key.endswith("_" + n) or n.endswith("_" + key):
                return value
    return None


def make_material(name, tex):
    mat = bpy.data.materials.new(name)
    mat.use_nodes = True
    nt = mat.node_tree
    bsdf = next(n for n in nt.nodes if n.bl_idname == 'ShaderNodeBsdfPrincipled')

    def image(path, colorspace):
        node = nt.nodes.new('ShaderNodeTexImage')
        node.image = bpy.data.images.load(path, check_existing=True)
        node.image.colorspace_settings.name = colorspace
        return node

    if "alb" in tex:
        alb = image(tex["alb"], 'sRGB')
        nt.links.new(alb.outputs['Color'], bsdf.inputs['Base Color'])
        nt.links.new(alb.outputs['Alpha'], bsdf.inputs['Alpha'])
    if "nrm" in tex:
        nrm = image(tex["nrm"], 'Non-Color')
        nmap = nt.nodes.new('ShaderNodeNormalMap')
        nt.links.new(nrm.outputs['Color'], nmap.inputs['Color'])
        nt.links.new(nmap.outputs['Normal'], bsdf.inputs['Normal'])
    if "orm" in tex:
        orm = image(tex["orm"], 'Non-Color')
        sep = nt.nodes.new('ShaderNodeSeparateColor')
        nt.links.new(orm.outputs['Color'], sep.inputs['Color'])
        nt.links.new(sep.outputs[1], bsdf.inputs['Roughness'])
        nt.links.new(sep.outputs[2], bsdf.inputs['Metallic'])
    return mat


def has_image(mat):
    return mat.use_nodes and any(n.bl_idname == 'ShaderNodeTexImage' and n.image for n in mat.node_tree.nodes)


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--models", required=True)
    p.add_argument("--textures", default="")
    p.add_argument("--include", default="", help="regular expression on file names to import")
    p.add_argument("--out", required=True)
    p.add_argument("--lod", type=int, default=0, help="keep only meshes of this level of detail")
    args = p.parse_args(common.script_args())

    common.reset_scene()
    table = index_textures(args.textures) if args.textures else {}
    include = re.compile(args.include, re.I) if args.include else None
    files = sorted(f for f in os.listdir(args.models) if os.path.splitext(f)[1].lower() in common.IMPORTERS)
    lod_re = re.compile(r"_lod(\d+)", re.I)
    for f in files:
        m = lod_re.search(f)
        if m and int(m.group(1)) != args.lod:
            continue
        if include and not include.search(f):
            continue
        objects = common.import_model(os.path.join(args.models, f))
        for obj in objects:
            if obj.type != 'MESH':
                continue
            slots = obj.material_slots or []
            if slots and all(s.material and has_image(s.material) for s in slots):
                continue  # already textured (glTF from d2rconvert)
            if not table:
                continue
            names = [s.material.name for s in slots if s.material] or [obj.name]
            for i, mname in enumerate(names):
                tex = find_set(table, mname, obj.name, os.path.splitext(f)[0])
                if tex is None:
                    print(f"no texture for {obj.name}/{mname}")
                    continue
                mat = make_material(mname, tex)
                if i < len(obj.data.materials):
                    obj.data.materials[i] = mat
                else:
                    obj.data.materials.append(mat)
    objects = list(bpy.context.scene.objects)
    if objects:
        common.ground(objects)
    os.makedirs(os.path.dirname(os.path.abspath(args.out)), exist_ok=True)
    bpy.ops.wm.save_as_mainfile(filepath=os.path.abspath(args.out))
    print(f"saved {args.out}")


main()
