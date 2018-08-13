package diablo3

import "bytes"

func newRoot(rootHash []byte, extract func(contentHash []byte) ([]byte, error)) (*Root, error) {
	rootB, err := extract(rootHash)
	if err != nil {
		return nil, err
	}
	d3root, err := parseD3RootFile(bytes.NewReader(rootB))
	if err != nil {
		return nil, err
	}
	filenameToContentHash := map[string][]byte{}
	for _, entry := range d3root.NamedEntries {
		// fmt.Printf("getting \"%s\" with hash %x\n", entry.Filename, entry.ContentKey)
		if entry.Filename == "Windows" || entry.Filename == "Mac" {
			// Those files cannot be downloaded for some reason
			continue
		}
		filenameToContentHash[entry.Filename] = entry.ContentKey[:]
		// file, err := r.Extract(entry.ContentKey[:])
		// if err != nil {
		// 	return err
		// }
		// fmt.Printf("%s len is: %s\n", entry.Filename, size(len(file)))
	}
	return &Root{
		filenameToContentHash: filenameToContentHash,
	}, nil
}
