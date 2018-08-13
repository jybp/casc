package diablo3

import "bytes"

func (r *Root) setup() error {
	if r.filenameToContentHash != nil {
		return nil
	}
	r.filenameToContentHash = map[string][]byte{}

	rootB, err := r.Extract(r.RootHash)
	if err != nil {
		return err
	}
	d3root, err := parseD3RootFile(bytes.NewReader(rootB))
	if err != nil {
		return err
	}
	for _, entry := range d3root.NamedEntries {
		// fmt.Printf("getting \"%s\" with hash %x\n", entry.Filename, entry.ContentKey)
		if entry.Filename == "Windows" || entry.Filename == "Mac" {
			// Those files cannot be downloaded for some reason
			continue
		}

		r.filenameToContentHash[entry.Filename] = entry.ContentKey[:]

		// file, err := r.Extract(entry.ContentKey[:])
		// if err != nil {
		// 	return err
		// }
		// fmt.Printf("%s len is: %s\n", entry.Filename, size(len(file)))
	}
	return nil
}
