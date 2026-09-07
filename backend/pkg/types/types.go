package types

type File struct {
	Path string
	Size int64
	Ext  string
	Head []byte
}

type Recipe struct {
	Codec 	string
	Version int
	Params  map[string]string
	Blob 	[]byte
}