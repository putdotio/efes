package main

import (
	"io"
	"net/http"
	"os"
	"time"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"golang.org/x/net/context"
)

func mount(cfg *Config, mountpoint string) error {
	clt, err := NewClient(cfg)
	if err != nil {
		return err
	}
	c, err := fuse.Mount(mountpoint)
	if err != nil {
		return err
	}
	defer c.Close()

	filesys := &FS{
		client: clt,
	}
	if err := fs.Serve(c, filesys); err != nil {
		return err
	}
	// check if the mount process has an error to report
	<-c.Ready
	if err := c.MountError; err != nil {
		return err
	}
	return nil
}

type FS struct {
	client *Client
}

var _ fs.FS = (*FS)(nil)

func (f *FS) Root() (fs.Node, error) {
	n := &Root{
		client: f.client,
	}
	return n, nil
}

type Root struct {
	client *Client
}

var _ fs.Node = (*Root)(nil)

func (d *Root) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Mode = os.ModeDir | 0310
	return nil
}

var _ = fs.NodeRequestLookuper(&Root{})

func (d *Root) Lookup(ctx context.Context, req *fuse.LookupRequest, resp *fuse.LookupResponse) (fs.Node, error) {
	key := req.Name
	remotePath, err := d.client.getPath(key)
	if cerr, ok := err.(*ClientError); ok && cerr.Code == http.StatusNotFound {
		return nil, fuse.ENOENT
	}
	if err != nil {
		return nil, err
	}
	r, err := d.client.httpClient.Head(remotePath.Path)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	if r.StatusCode == http.StatusNotFound {
		return nil, fuse.ENOENT
	}
	err = checkResponseError(r)
	if err != nil {
		return nil, err
	}
	t, _ := time.Parse(time.RFC3339, remotePath.CreatedAt)
	f := &File{
		client:    d.client,
		key:       key,
		createdAt: t,
		size:      uint64(d.client.getContentLength(r)),
	}
	return f, nil
}

var _ = fs.HandleReadDirAller(&Root{})

func (d *Root) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	var res []fuse.Dirent
	return res, nil
}

type File struct {
	client    *Client
	key       string
	createdAt time.Time
	size      uint64
}

var _ fs.Node = (*File)(nil)

func (f *File) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Size = f.size
	a.Mode = 0640
	a.Mtime = f.createdAt
	a.Ctime = f.createdAt
	a.Crtime = f.createdAt
	return nil
}

var _ = fs.NodeOpener(&File{})

func (f *File) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	remotePath, err := f.client.getPath(f.key)
	if err != nil {
		return nil, err
	}
	r, err := f.client.httpClient.Get(remotePath.Path)
	if err != nil {
		return nil, err
	}
	err = checkResponseError(r)
	if err != nil {
		return nil, err
	}
	// individual entries inside a zip file are not seekable
	resp.Flags |= fuse.OpenNonSeekable
	return &FileHandle{r: r.Body}, nil
}

type FileHandle struct {
	r io.ReadCloser
}

var _ fs.Handle = (*FileHandle)(nil)

var _ fs.HandleReleaser = (*FileHandle)(nil)

func (fh *FileHandle) Release(ctx context.Context, req *fuse.ReleaseRequest) error {
	return fh.r.Close()
}

var _ = fs.HandleReader(&FileHandle{})

func (fh *FileHandle) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	// We don't actually enforce Offset to match where previous read
	// ended. Maybe we should, but that would mean'd we need to track
	// it. The kernel *should* do it for us, based on the
	// fuse.OpenNonSeekable flag.
	//
	// One exception to the above is if we fail to fully populate a
	// page cache page; a read into page cache is always page aligned.
	// Make sure we never serve a partial read, to avoid that.
	buf := make([]byte, req.Size)
	n, err := io.ReadFull(fh.r, buf)
	if err == io.ErrUnexpectedEOF || err == io.EOF {
		err = nil
	}
	resp.Data = buf[:n]
	return err
}
