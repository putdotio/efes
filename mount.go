package main

import (
	"context"
	"io"
	"net/http"
	"os"
	"strconv"
	"syscall"
	"time"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
)

func mount(cfg *Config, mountpoint string) error {
	clt, err := NewClient(cfg)
	if err != nil {
		return err
	}
	mountOptions := []fuse.MountOption{
		fuse.AllowOther(),
		fuse.NoAppleDouble(),
		fuse.NoAppleXattr(),
		fuse.ReadOnly(),
	}
	c, err := fuse.Mount(mountpoint, mountOptions...)
	if err != nil {
		return err
	}
	defer c.Close() // nolint: errcheck

	filesys := &FS{client: clt}
	if err := fs.Serve(c, filesys); err != nil {
		return err
	}
	<-c.Ready
	return c.MountError
}

type FS struct {
	client *Client
}

var _ fs.FS = (*FS)(nil)

func (f *FS) Root() (fs.Node, error) {
	return &Root{client: f.client}, nil
}

type Root struct {
	client *Client
}

var _ fs.Node = (*Root)(nil)

func (r *Root) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Mode = os.ModeDir | 0111
	return nil
}

var _ fs.NodeRequestLookuper = (*Root)(nil)

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
	defer r.Body.Close() // nolint: errcheck
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
		path:      remotePath.Path,
		createdAt: t,
		size:      uint64(d.client.getContentLength(r)),
	}
	return f, nil
}

type File struct {
	client    *Client
	path      string
	createdAt time.Time
	size      uint64
}

var _ fs.Node = (*File)(nil)

func (f *File) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Size = f.size
	a.Mode = 0444
	a.Mtime = f.createdAt
	a.Ctime = f.createdAt
	a.Crtime = f.createdAt
	return nil
}

var _ fs.NodeOpener = (*File)(nil)

func (f *File) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	if !req.Flags.IsReadOnly() {
		return nil, fuse.Errno(syscall.EROFS)
	}
	return &FileHandle{
		client: f.client,
		path:   f.path,
		size:   f.size,
	}, nil
}

type FileHandle struct {
	client *Client
	path   string
	size   uint64
	r      io.ReadCloser
	offset int64
}

var _ fs.Handle = (*FileHandle)(nil)

func (h *FileHandle) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	if req.Offset != h.offset {
		err := h.close(ctx)
		if err != nil {
			return err
		}
	}
	if h.r == nil {
		if err := h.open(ctx, req.Offset); err != nil {
			return err
		}
	}
	buf := make([]byte, req.Size)
	n, err := io.ReadFull(h.r, buf)
	if err == io.ErrUnexpectedEOF || err == io.EOF {
		err = nil
	}
	resp.Data = buf[:n]
	h.offset += int64(n)
	return err
}

func (h *FileHandle) open(ctx context.Context, offset int64) error {
	req, err := http.NewRequest(http.MethodGet, h.path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("bytes", strconv.FormatInt(h.offset, 10)+"-")
	r, err := h.client.httpClient.Do(req)
	if err != nil {
		return err
	}
	err = checkResponseError(r)
	if err != nil {
		return err
	}
	h.r = r.Body
	h.offset = offset
	return nil
}

var _ fs.HandleReleaser = (*FileHandle)(nil)

func (h *FileHandle) Release(ctx context.Context, req *fuse.ReleaseRequest) error {
	return h.close(ctx)
}

func (h *FileHandle) close(ctx context.Context) error {
	if h.r == nil {
		return nil
	}
	err := h.r.Close()
	h.r = nil
	return err
}
