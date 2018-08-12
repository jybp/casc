package casc

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/jybp/casc/blte"
	"github.com/jybp/casc/common"
)

type extractor struct {
	downloader   Downloader
	cdn          common.Cdn
	encoding     common.Encoding
	archivesIdxs map[string][]common.ArchiveIndexEntry
}

func (s *extractor) extract(contentHash []byte) ([]byte, error) {
	encodedHash, err := s.encoding.FindEncodedHash(contentHash)
	if err != nil {
		return nil, err
	}

	// fmt.Printf("encoded hash for decoded hash %x is %x\n", contentHash, encodedHash)

	archiveInfo := struct {
		ArchiveHash string
		Index       common.ArchiveIndexEntry
	}{}
	for archiveHash, indices := range s.archivesIdxs {
		for _, idx := range indices {
			if bytes.Compare(encodedHash, idx.HeaderHash[:]) == 0 {
				archiveInfo = struct {
					ArchiveHash string
					Index       common.ArchiveIndexEntry
				}{archiveHash, idx}
				break
			}
		}
	}

	if archiveInfo.ArchiveHash == "" || archiveInfo.Index == (common.ArchiveIndexEntry{}) {
		// encodedHash was not found inside archive indices, try to download the whole file
		r, err := s.downloader.Get(common.Url(s.cdn.Hosts[0], s.cdn.Path, common.PathTypeData, fmt.Sprintf("%x", encodedHash), false))
		if err != nil {
			return nil, err
		}
		return ioutil.ReadAll(r)
	}

	// TODO should only download relevant part of the archive using
	// http header Content-Range bytes start-end/total:
	//  1. check file exist
	//  2. check file size is >= offset+size
	//  3. check the content of file offset-size is not just 0 padded
	//
	//  To store downloaded content-range open the file and write
	//  the content to the correct offset.
	archive, err := s.downloader.Get(common.Url(s.cdn.Hosts[0], s.cdn.Path, common.PathTypeData, archiveInfo.ArchiveHash, false))
	if err != nil {
		return nil, err
	}
	if _, err := archive.Seek(int64(archiveInfo.Index.Offset), 0); err != nil {
		return nil, err
	}
	encodedFile := make([]byte, archiveInfo.Index.EncodedSize)
	if _, err := io.ReadFull(archive, encodedFile); err != nil {
		return nil, err
	}
	decodedFile := bytes.NewBuffer([]byte{})
	if err := blte.Decode(bytes.NewBuffer(encodedFile), decodedFile); err != nil {
		return nil, err
	}
	return decodedFile.Bytes(), nil
}
