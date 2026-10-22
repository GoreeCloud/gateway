package responsecoding

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"

	"github.com/GoreeCloud/goreecloud-gateway/internal/config"
	"github.com/klauspost/compress/zstd"
)

// EncodeKnownLengthProxy is the deliberately narrow first streaming proxy
// encoder. It does not prebuffer an upstream response: the source is consumed
// only as the client consumes the encoded pipe, preserving backpressure. It
// fails closed to identity for unknown lengths, protocol upgrades, SSE,
// incremental flushes, ranges, existing codings and unverified semantics.
func EncodeKnownLengthProxy(req *http.Request, resp *http.Response, policy *config.ResponseEncoding) error {
	if policy == nil || resp == nil {
		return nil
	}
	if resp.Header == nil {
		resp.Header = make(http.Header)
	}
	addVary(resp.Header, "Accept-Encoding")
	if req.Method != http.MethodGet || resp.StatusCode != http.StatusOK || resp.Body == nil ||
		resp.ContentLength < 0 || resp.Uncompressed || req.Header.Get("Range") != "" ||
		req.Header.Get("Upgrade") != "" || resp.Header.Get("Content-Range") != "" ||
		resp.Header.Get("Content-Encoding") != "" || len(resp.Trailer) > 0 ||
		resp.Header.Get("Trailer") != "" || resp.Header.Get("Incremental") != "" ||
		hasDirective(strings.Join(req.Header.Values("Cache-Control"), ","), "no-transform") ||
		hasDirective(strings.Join(resp.Header.Values("Cache-Control"), ","), "no-transform") ||
		!compressibleType(resp.Header.Get("Content-Type")) ||
		strings.HasPrefix(strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type"))), "text/event-stream") {
		return nil
	}
	minimumLength := DefaultMinimumLength
	if policy.MinimumLength != nil {
		minimumLength = *policy.MinimumLength
	}
	if resp.ContentLength < int64(minimumLength) || resp.ContentLength == 0 {
		return nil
	}
	format := negotiate(req.Header.Values("Accept-Encoding"), policy.Formats)
	if format == "" {
		return nil
	}

	reader, writer := io.Pipe()
	var compressor io.WriteCloser
	switch format {
	case "gzip":
		compressor = gzip.NewWriter(writer)
	case "zstd":
		encoder, err := zstd.NewWriter(writer)
		if err != nil {
			_ = reader.Close()
			_ = writer.Close()
			return err
		}
		compressor = encoder
	default:
		_ = reader.Close()
		_ = writer.Close()
		return nil // Validation forbids unknown encodings; direct calls fail closed.
	}
	original := resp.Body
	resp.Body = &encodedProxyBody{PipeReader: reader, original: original}
	resp.ContentLength = -1
	resp.Header.Del("Content-Length")
	resp.Header.Set("Content-Encoding", format)
	resp.Header.Del("ETag")
	resp.Header.Del("Content-Md5")
	resp.Header.Del("Digest")
	resp.Header.Del("Content-Digest")
	resp.Header.Del("Accept-Ranges")
	go func() {
		defer original.Close()
		_, copyErr := io.Copy(compressor, original)
		closeErr := compressor.Close()
		if copyErr != nil {
			_ = writer.CloseWithError(copyErr)
			return
		}
		_ = writer.CloseWithError(closeErr)
	}()
	return nil
}

type encodedProxyBody struct {
	*io.PipeReader
	original io.ReadCloser
}

func (b *encodedProxyBody) Close() error {
	pipeErr := b.PipeReader.Close()
	upstreamErr := b.original.Close()
	if pipeErr != nil {
		return pipeErr
	}
	return upstreamErr
}
