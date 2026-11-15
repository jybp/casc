package granny

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/pkg/errors"
)

// TextureLoader resolves a Granny texture file name (i.e.
// "data/hd/character/enemy/x/textures/x_alb.texture") to either PNG bytes to embed or a
// URI to reference. Returning neither skips the texture.
type TextureLoader func(fromFileName string) (png []byte, uri string, err error)

// ExportOptions configures ExportGLB.
type ExportOptions struct {
	Textures TextureLoader // nil: no textures
	Skin     bool          // export the skeleton and skinning weights
}

type gltfDoc struct {
	Asset       map[string]string        `json:"asset"`
	Scene       int                      `json:"scene"`
	Scenes      []map[string]interface{} `json:"scenes"`
	Nodes       []map[string]interface{} `json:"nodes"`
	Meshes      []map[string]interface{} `json:"meshes,omitempty"`
	Materials   []map[string]interface{} `json:"materials,omitempty"`
	Textures    []map[string]interface{} `json:"textures,omitempty"`
	Images      []map[string]interface{} `json:"images,omitempty"`
	Samplers    []map[string]interface{} `json:"samplers,omitempty"`
	Skins       []map[string]interface{} `json:"skins,omitempty"`
	Accessors   []map[string]interface{} `json:"accessors"`
	BufferViews []map[string]interface{} `json:"bufferViews"`
	Buffers     []map[string]interface{} `json:"buffers"`
}

type gltfBuilder struct {
	doc      gltfDoc
	bin      bytes.Buffer
	images   map[string]int // texture file name -> texture index
	opts     ExportOptions
	warnings []string
}

func (g *gltfBuilder) view(data []byte, target int) int {
	for g.bin.Len()%4 != 0 {
		g.bin.WriteByte(0)
	}
	v := map[string]interface{}{"buffer": 0, "byteOffset": g.bin.Len(), "byteLength": len(data)}
	if target != 0 {
		v["target"] = target
	}
	g.bin.Write(data)
	g.doc.BufferViews = append(g.doc.BufferViews, v)
	return len(g.doc.BufferViews) - 1
}

func (g *gltfBuilder) accessor(data []byte, target, componentType, count int, typ string, extra map[string]interface{}) int {
	a := map[string]interface{}{
		"bufferView":    g.view(data, target),
		"componentType": componentType,
		"count":         count,
		"type":          typ,
	}
	for k, v := range extra {
		a[k] = v
	}
	g.doc.Accessors = append(g.doc.Accessors, a)
	return len(g.doc.Accessors) - 1
}

func f32s(v []float32) []byte {
	b := make([]byte, 4*len(v))
	for i, x := range v {
		binary.LittleEndian.PutUint32(b[4*i:], math.Float32bits(x))
	}
	return b
}

func objects(v interface{}) []*Object {
	switch x := v.(type) {
	case []*Object:
		return x
	case *VariantArray:
		o, _ := x.Objects()
		return o
	}
	return nil
}

func num(v interface{}) float64 {
	f, _ := v.(float64)
	return f
}

func str(v interface{}) string {
	s, _ := v.(string)
	return s
}

// texture returns the glTF texture index for a Granny texture file name, loading it once.
func (g *gltfBuilder) texture(name string) (int, bool) {
	if g.opts.Textures == nil || name == "" {
		return 0, false
	}
	if i, ok := g.images[name]; ok {
		return i, i >= 0
	}
	png, uri, err := g.opts.Textures(name)
	if err != nil || (png == nil && uri == "") {
		if err != nil {
			g.warnings = append(g.warnings, fmt.Sprintf("texture %s: %v", name, err))
		}
		g.images[name] = -1
		return 0, false
	}
	if len(g.doc.Samplers) == 0 {
		g.doc.Samplers = append(g.doc.Samplers, map[string]interface{}{"magFilter": 9729, "minFilter": 9987})
	}
	if png != nil {
		g.doc.Images = append(g.doc.Images, map[string]interface{}{
			"bufferView": g.view(png, 0), "mimeType": "image/png", "name": name,
		})
	} else {
		g.doc.Images = append(g.doc.Images, map[string]interface{}{"uri": uri, "name": name})
	}
	g.doc.Textures = append(g.doc.Textures, map[string]interface{}{"source": len(g.doc.Images) - 1, "sampler": 0})
	i := len(g.doc.Textures) - 1
	g.images[name] = i
	return i, true
}

// textureNames collects the texture file names reachable from a Granny material.
func textureNames(m *Object, depth int, out map[string]bool) {
	if m == nil || depth > 4 {
		return
	}
	if t, ok := m.Get("Texture").(*Object); ok {
		if n := str(t.Get("FromFileName")); n != "" {
			out[n] = true
		}
	}
	for _, mp := range objects(m.Get("Maps")) {
		if sub, ok := mp.Get("Map").(*Object); ok {
			textureNames(sub, depth+1, out)
		}
	}
}

// kind returns alb, nrm, orm... from a texture file name.
func kind(name string) string {
	n := strings.ToLower(name)
	n = strings.TrimSuffix(n, ".texture")
	if i := strings.LastIndex(n, "_"); i >= 0 {
		k := n[i+1:]
		if j := strings.IndexByte(k, '$'); j >= 0 {
			k = k[:j]
		}
		return k
	}
	return ""
}

func (g *gltfBuilder) material(m *Object, cache map[*Object]int) int {
	if i, ok := cache[m]; ok {
		return i
	}
	mat := map[string]interface{}{"name": str(m.Get("Name"))}
	pbr := map[string]interface{}{"metallicFactor": 1.0, "roughnessFactor": 1.0}
	names := map[string]bool{}
	textureNames(m, 0, names)
	hasORM := false
	for n := range names {
		idx, ok := g.texture(n)
		if !ok {
			continue
		}
		switch kind(n) {
		case "alb":
			pbr["baseColorTexture"] = map[string]interface{}{"index": idx}
			mat["alphaMode"] = "MASK"
			mat["alphaCutoff"] = 0.5
		case "nrm":
			mat["normalTexture"] = map[string]interface{}{"index": idx}
		case "orm":
			// Same channel layout as glTF: R occlusion, G roughness, B metalness.
			pbr["metallicRoughnessTexture"] = map[string]interface{}{"index": idx}
			mat["occlusionTexture"] = map[string]interface{}{"index": idx}
			hasORM = true
		}
	}
	if !hasORM {
		pbr["metallicFactor"] = 0.0
		pbr["roughnessFactor"] = 0.8
	}
	mat["pbrMetallicRoughness"] = pbr
	mat["doubleSided"] = true
	g.doc.Materials = append(g.doc.Materials, mat)
	i := len(g.doc.Materials) - 1
	cache[m] = i
	return i
}

// ExportGLB converts the meshes (and optionally the skeleton) of a Granny file to binary glTF.
// Units are converted to meters using the art tool information.
func ExportGLB(f *File, opts ExportOptions) ([]byte, []string, error) {
	g := &gltfBuilder{images: map[string]int{}, opts: opts}
	g.doc.Asset = map[string]string{"version": "2.0", "generator": "casc/granny"}
	unitsPerMeter := 1.0
	if art, ok := f.Root.Get("ArtToolInfo").(*Object); ok {
		if u := num(art.Get("UnitsPerMeter")); u > 0 {
			unitsPerMeter = u
		}
	}
	s := 1 / unitsPerMeter
	g.doc.Nodes = append(g.doc.Nodes, map[string]interface{}{"name": "root", "scale": []float64{s, s, s}})
	rootChildren := []int{}

	// Skeleton.
	boneIndex := map[string]int{}
	skin := -1
	if opts.Skin {
		if sks := objects(f.Root.Get("Skeletons")); len(sks) > 0 {
			bones := objects(sks[0].Get("Bones"))
			first := len(g.doc.Nodes)
			ibm := make([]float32, 0, 16*len(bones))
			joints := make([]int, len(bones))
			for i, b := range bones {
				name := str(b.Get("Name"))
				boneIndex[name] = i
				node := map[string]interface{}{"name": name}
				if t, ok := b.Get("Transform").(Transform); ok {
					node["translation"] = []float32{t.Position[0], t.Position[1], t.Position[2]}
					q := t.Orientation
					if q != [4]float32{} {
						node["rotation"] = []float32{q[0], q[1], q[2], q[3]}
					}
					node["scale"] = []float32{t.ScaleShear[0], t.ScaleShear[4], t.ScaleShear[8]}
				}
				g.doc.Nodes = append(g.doc.Nodes, node)
				joints[i] = first + i
				m, _ := b.Get("InverseWorldTransform").([]float64)
				for k := 0; k < 16; k++ {
					v := float32(0)
					if k < len(m) {
						v = float32(m[k])
					} else if k%5 == 0 {
						v = 1
					}
					ibm = append(ibm, v)
				}
			}
			for i, b := range bones {
				p := int(num(b.Get("ParentIndex")))
				if p >= 0 && p < len(bones) {
					parent := g.doc.Nodes[first+p]
					c, _ := parent["children"].([]int)
					parent["children"] = append(c, first+i)
				} else {
					rootChildren = append(rootChildren, first+i)
				}
			}
			if len(bones) > 0 {
				acc := g.accessor(f32s(ibm), 0, 5126, len(bones), "MAT4", nil)
				g.doc.Skins = append(g.doc.Skins, map[string]interface{}{
					"joints": joints, "inverseBindMatrices": acc, "skeleton": first,
				})
				skin = 0
			}
		}
	}

	matCache := map[*Object]int{}
	for _, mesh := range objects(f.Root.Get("Meshes")) {
		prims, err := g.mesh(mesh, matCache, boneIndex, skin >= 0)
		if err != nil {
			g.warnings = append(g.warnings, fmt.Sprintf("mesh %s: %v", str(mesh.Get("Name")), err))
			continue
		}
		if len(prims) == 0 {
			continue
		}
		g.doc.Meshes = append(g.doc.Meshes, map[string]interface{}{"name": str(mesh.Get("Name")), "primitives": prims})
		node := map[string]interface{}{"name": str(mesh.Get("Name")), "mesh": len(g.doc.Meshes) - 1}
		if skin >= 0 {
			node["skin"] = skin
		}
		g.doc.Nodes = append(g.doc.Nodes, node)
		rootChildren = append(rootChildren, len(g.doc.Nodes)-1)
	}
	if len(g.doc.Meshes) == 0 && skin < 0 {
		return nil, g.warnings, errors.New("granny: nothing to export")
	}
	g.doc.Nodes[0]["children"] = rootChildren
	g.doc.Scenes = []map[string]interface{}{{"nodes": []int{0}}}
	for g.bin.Len()%4 != 0 {
		g.bin.WriteByte(0)
	}
	g.doc.Buffers = []map[string]interface{}{{"byteLength": g.bin.Len()}}
	js, err := json.Marshal(g.doc)
	if err != nil {
		return nil, g.warnings, errors.WithStack(err)
	}
	for len(js)%4 != 0 {
		js = append(js, ' ')
	}
	var out bytes.Buffer
	total := 12 + 8 + len(js) + 8 + g.bin.Len()
	binary.Write(&out, binary.LittleEndian, []uint32{0x46546C67, 2, uint32(total)})
	binary.Write(&out, binary.LittleEndian, []uint32{uint32(len(js)), 0x4E4F534A})
	out.Write(js)
	binary.Write(&out, binary.LittleEndian, []uint32{uint32(g.bin.Len()), 0x004E4942})
	out.Write(g.bin.Bytes())
	return out.Bytes(), g.warnings, nil
}

func (g *gltfBuilder) mesh(mesh *Object, matCache map[*Object]int, boneIndex map[string]int, skinned bool) ([]map[string]interface{}, error) {
	vd, _ := mesh.Get("PrimaryVertexData").(*Object)
	topo, _ := mesh.Get("PrimaryTopology").(*Object)
	if vd == nil || topo == nil {
		return nil, errors.New("no vertex data or topology")
	}
	va, _ := vd.Get("Vertices").(*VariantArray)
	if va == nil || va.Count == 0 {
		return nil, errors.New("no vertices")
	}
	n := va.Count
	scale := 1.0
	if ed, ok := mesh.Get("ExtendedData").(*Object); ok {
		if v := num(ed.Get("VertexScale")); v > 0 {
			scale = v
		}
	}
	attrs := map[string]interface{}{}

	// Positions: float, or normalized integers scaled by VertexScale.
	posOff, posM, ok := va.Field("Position")
	if !ok {
		return nil, errors.New("no positions")
	}
	normalized := posM.Type != tFloat && posM.Type != tHalf
	pos := make([]float32, 0, 3*n)
	min := []float32{math.MaxFloat32, math.MaxFloat32, math.MaxFloat32}
	max := []float32{-math.MaxFloat32, -math.MaxFloat32, -math.MaxFloat32}
	for i := 0; i < n; i++ {
		v := va.Floats(i, posOff, posM)
		for k := 0; k < 3; k++ {
			x := float32(0)
			if k < len(v) {
				x = float32(v[k])
				if normalized {
					x = float32(v[k] * scale)
				}
			}
			pos = append(pos, x)
			if x < min[k] {
				min[k] = x
			}
			if x > max[k] {
				max[k] = x
			}
		}
	}
	attrs["POSITION"] = g.accessor(f32s(pos), 34962, 5126, n, "VEC3", map[string]interface{}{"min": min, "max": max})

	if off, m, ok := va.Field("Normal"); ok {
		nrm := make([]float32, 0, 3*n)
		for i := 0; i < n; i++ {
			v := va.Floats(i, off, m)
			x, y, z := float64(0), float64(1), float64(0)
			if len(v) >= 3 {
				x, y, z = v[0], v[1], v[2]
			}
			l := math.Sqrt(x*x + y*y + z*z)
			if l == 0 {
				x, y, z, l = 0, 1, 0, 1
			}
			nrm = append(nrm, float32(x/l), float32(y/l), float32(z/l))
		}
		attrs["NORMAL"] = g.accessor(f32s(nrm), 34962, 5126, n, "VEC3", nil)
	}
	if off, m, ok := va.Field("TextureCoordinates0"); ok {
		uv := make([]float32, 0, 2*n)
		for i := 0; i < n; i++ {
			v := va.Floats(i, off, m)
			u, w := float32(0), float32(0)
			if len(v) >= 2 {
				u, w = float32(v[0]), float32(v[1])
			}
			uv = append(uv, u, w)
		}
		attrs["TEXCOORD_0"] = g.accessor(f32s(uv), 34962, 5126, n, "VEC2", nil)
	}
	if skinned {
		wOff, wM, okW := va.Field("BoneWeights")
		iOff, iM, okI := va.Field("BoneIndices")
		bindings := objects(mesh.Get("BoneBindings"))
		if okW && okI && len(bindings) > 0 {
			remap := make([]int, len(bindings))
			for i, b := range bindings {
				remap[i] = boneIndex[str(b.Get("BoneName"))]
			}
			joints := make([]byte, 0, 8*n)
			weights := make([]float32, 0, 4*n)
			for i := 0; i < n; i++ {
				w := va.Floats(i, wOff, wM)
				idx := va.Floats(i, iOff, iM)
				sum := 0.0
				for k := 0; k < 4 && k < len(w); k++ {
					sum += w[k]
				}
				for k := 0; k < 4; k++ {
					j, wt := 0, 0.0
					if k < len(idx) && k < len(w) {
						if bi := int(idx[k]); bi >= 0 && bi < len(remap) {
							j = remap[bi]
						}
						wt = w[k]
					}
					if sum > 0 {
						wt /= sum
					} else if k == 0 {
						wt = 1
					}
					joints = append(joints, byte(j), byte(j>>8))
					weights = append(weights, float32(wt))
				}
			}
			attrs["JOINTS_0"] = g.accessor(joints, 34962, 5123, n, "VEC4", nil)
			attrs["WEIGHTS_0"] = g.accessor(f32s(weights), 34962, 5126, n, "VEC4", nil)
		}
	}

	// Indices.
	var indices []uint32
	if ia, ok := topo.Get("Indices").(*VariantArray); ok && ia.Count > 0 {
		for i := 0; i < ia.Count; i++ {
			indices = append(indices, binary.LittleEndian.Uint32(ia.Data[i*ia.Stride:]))
		}
	} else if ia, ok := topo.Get("Indices16").(*VariantArray); ok && ia.Count > 0 {
		for i := 0; i < ia.Count; i++ {
			indices = append(indices, uint32(binary.LittleEndian.Uint16(ia.Data[i*ia.Stride:])))
		}
	}
	if len(indices) == 0 {
		return nil, errors.New("no indices")
	}
	materials := objects(mesh.Get("MaterialBindings"))
	groups := objects(topo.Get("Groups"))
	if len(groups) == 0 {
		groups = []*Object{{Fields: map[string]interface{}{"MaterialIndex": 0.0, "TriFirst": 0.0, "TriCount": float64(len(indices) / 3)}}}
	}
	var prims []map[string]interface{}
	for _, gr := range groups {
		first, count := int(num(gr.Get("TriFirst"))), int(num(gr.Get("TriCount")))
		if first < 0 || count <= 0 || (first+count)*3 > len(indices) {
			continue
		}
		b := make([]byte, 4*3*count)
		for i, ix := range indices[first*3 : (first+count)*3] {
			if int(ix) >= n {
				return nil, errors.Errorf("index %d out of range", ix)
			}
			binary.LittleEndian.PutUint32(b[4*i:], ix)
		}
		prim := map[string]interface{}{
			"attributes": attrs,
			"indices":    g.accessor(b, 34963, 5125, 3*count, "SCALAR", nil),
		}
		if mi := int(num(gr.Get("MaterialIndex"))); mi >= 0 && mi < len(materials) {
			if mat, ok := materials[mi].Get("Material").(*Object); ok && mat != nil {
				prim["material"] = g.material(mat, matCache)
			}
		}
		prims = append(prims, prim)
	}
	return prims, nil
}
