"""Shared Blender helpers for the reference scripts (run inside Blender: blender -b -P script.py -- args)."""
import json
import math
import os
import sys

import bpy
from mathutils import Vector

IMPORTERS = {
    ".glb": lambda p: bpy.ops.import_scene.gltf(filepath=p),
    ".gltf": lambda p: bpy.ops.import_scene.gltf(filepath=p),
    ".fbx": lambda p: bpy.ops.import_scene.fbx(filepath=p),
    ".obj": lambda p: bpy.ops.wm.obj_import(filepath=p),
    ".blend": None,  # handled by append_blend
}
if hasattr(bpy.ops.wm, "collada_import"):  # removed in Blender 5
    IMPORTERS[".dae"] = lambda p: bpy.ops.wm.collada_import(filepath=p)


def script_args():
    """Arguments after '--' on the Blender command line."""
    return sys.argv[sys.argv.index("--") + 1:] if "--" in sys.argv else []


def load_rig(path):
    with open(path, "r", encoding="utf-8") as f:
        return json.load(f)


def reset_scene():
    bpy.ops.wm.read_factory_settings(use_empty=True)
    return bpy.context.scene


def import_model(path):
    """Imports a model file and returns the new mesh objects."""
    before = set(bpy.data.objects)
    ext = os.path.splitext(path)[1].lower()
    if ext == ".blend":
        with bpy.data.libraries.load(path, link=False) as (src, dst):
            dst.objects = src.objects
        for obj in dst.objects:
            if obj is not None:
                bpy.context.scene.collection.objects.link(obj)
    elif ext in IMPORTERS:
        IMPORTERS[ext](path)
    else:
        raise ValueError(f"unsupported model format {ext}")
    new = [o for o in bpy.data.objects if o not in before]
    # The glTF importer adds a display shape for bones (an icosphere): never render it.
    shapes = {pb.custom_shape for o in new if o.type == 'ARMATURE' for pb in o.pose.bones if pb.custom_shape}
    for o in list(new):
        if o in shapes or (o.type == 'MESH' and o.name.lower().startswith("icosphere") and not o.data.materials):
            new.remove(o)
            bpy.data.objects.remove(o, do_unlink=True)
    return new


def mesh_bounds(objects):
    """World space bounding box of the mesh objects."""
    lo = Vector((math.inf,) * 3)
    hi = Vector((-math.inf,) * 3)
    deps = bpy.context.evaluated_depsgraph_get()
    for obj in objects:
        if obj.type != 'MESH':
            continue
        ev = obj.evaluated_get(deps)
        for corner in ev.bound_box:
            p = ev.matrix_world @ Vector(corner)
            lo = Vector(map(min, lo, p))
            hi = Vector(map(max, hi, p))
    return lo, hi


def ground(objects):
    """Puts the models' feet on the origin (x/y centred, lowest point at z=0)."""
    roots = [o for o in objects if o.parent is None]
    lo, hi = mesh_bounds(objects)
    offset = Vector((-(lo.x + hi.x) / 2, -(lo.y + hi.y) / 2, -lo.z))
    for r in roots:
        r.location += offset
    bpy.context.view_layer.update()
    return mesh_bounds(objects)


def rotate_yaw(objects, yaw_deg):
    roots = [o for o in objects if o.parent is None]
    for r in roots:
        r.rotation_mode = 'XYZ'
        r.rotation_euler.z += math.radians(yaw_deg)
    bpy.context.view_layer.update()


def sun(name, spec):
    data = bpy.data.lights.new(name, 'SUN')
    data.color = spec["color"]
    data.energy = spec["strength"]
    data.angle = math.radians(spec.get("angle_deg", 8.0))
    obj = bpy.data.objects.new(name, data)
    obj.rotation_euler = tuple(math.radians(a) for a in spec["rotation_deg"])
    bpy.context.scene.collection.objects.link(obj)
    return obj


def world(scene, spec):
    w = bpy.data.worlds.new("World")
    scene.world = w
    w.use_nodes = True
    bg = next(n for n in w.node_tree.nodes if n.bl_idname == 'ShaderNodeBackground')
    bg.inputs[0].default_value = (*spec["color"], 1.0)
    bg.inputs[1].default_value = spec["strength"]


def light_rig(scene, rig):
    sun("Key", rig["key"])
    sun("Rim", rig["rim"])
    world(scene, rig["world"])


def side_camera(scene, rig, lo, hi):
    """Orthographic camera looking along +y, raised by the rig elevation, framing the bounds."""
    e = math.radians(rig["elevation_deg"])
    margin = rig.get("margin", 0.1)
    # Projected screen extent of the box: v = z*cos(e) + y*sin(e).
    corners = [Vector((x, y, z)) for x in (lo.x, hi.x) for y in (lo.y, hi.y) for z in (lo.z, hi.z)]
    vs = [c.z * math.cos(e) + c.y * math.sin(e) for c in corners]
    u_min, u_max = lo.x - margin, hi.x + margin
    v_min, v_max = min(vs) - margin, max(vs) + margin
    width, height = u_max - u_min, v_max - v_min
    data = bpy.data.cameras.new("Camera")
    data.type = 'ORTHO'
    data.ortho_scale = max(width, height)
    data.clip_start, data.clip_end = 0.01, 500.0
    cam = bpy.data.objects.new("Camera", data)
    scene.collection.objects.link(cam)
    vc = (v_min + v_max) / 2
    target = Vector(((u_min + u_max) / 2, vc * math.sin(e), vc * math.cos(e)))
    direction = Vector((0.0, math.cos(e), -math.sin(e)))
    cam.location = target - direction * 100.0
    cam.rotation_euler = (math.radians(90.0) - e, 0.0, 0.0)
    scene.camera = cam
    ppu = rig["pixels_per_unit"]
    scene.render.resolution_x = max(8, int(round(width * ppu)))
    scene.render.resolution_y = max(8, int(round(height * ppu)))
    scene.render.resolution_percentage = 100
    return cam


def render_settings(scene, rig):
    scene.render.engine = 'CYCLES'
    prefs = bpy.context.preferences.addons.get("cycles")
    device = os.environ.get("REF_DEVICE", rig.get("device", "auto")).lower()
    if device == "cpu":
        prefs = None
        scene.cycles.device = 'CPU'
    if prefs:
        cp = prefs.preferences
        for backend in ("HIP", "OPTIX", "CUDA", "METAL", "ONEAPI"):
            try:
                cp.compute_device_type = backend
                cp.get_devices()
                if any(d.type == backend for d in cp.devices):
                    for d in cp.devices:
                        d.use = d.type == backend
                    scene.cycles.device = 'GPU'
                    break
            except TypeError:
                continue
    scene.cycles.samples = rig.get("samples", 64)
    scene.cycles.use_denoising = True
    scene.render.film_transparent = True
    scene.render.image_settings.file_format = 'PNG'
    scene.render.image_settings.color_mode = 'RGBA'
    try:
        scene.view_settings.view_transform = rig.get("view_transform", "AgX")
    except TypeError:
        pass


def silhouette_material():
    mat = bpy.data.materials.new("Silhouette")
    mat.use_nodes = True
    nodes = mat.node_tree.nodes
    nodes.clear()
    em = nodes.new('ShaderNodeEmission')
    em.inputs[0].default_value = (0, 0, 0, 1)
    out = nodes.new('ShaderNodeOutputMaterial')
    mat.node_tree.links.new(em.outputs[0], out.inputs[0])
    return mat
