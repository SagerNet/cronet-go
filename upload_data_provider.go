package cronet

// #include <stdlib.h>
// #include <stdbool.h>
// #include <cronet_c.h>
import "C"

// UploadDataProvider
// The interface allowing the embedder to provide an upload body to
// URLRequest. It supports both non-chunked (size known in advanced) and
// chunked (size not known in advance) uploads. Be aware that not all servers
// support chunked uploads.
//
// An upload is either always chunked, across multiple uploads if the data
// ends up being sent more than once, or never chunked.
type UploadDataProvider struct {
	ptr C.Cronet_UploadDataProviderPtr
}

type UploadDataProviderHandler interface {
	// Length
	// If this is a non-chunked upload, returns the length of the upload. Must
	// always return -1 if this is a chunked upload.
	Length(self UploadDataProvider) int64

	// Read
	// Reads upload data into |buffer|. Each call of this method must be followed be a
	// single call, either synchronous or asynchronous, to
	// UploadDataSink.OnReadSucceeded() on success
	// or UploadDataSink.OnReadError() on failure. Neither read nor rewind
	// will be called until one of those methods or the other is called. Even if
	// the associated UrlRequest is canceled, one or the other must
	// still be called before resources can be safely freed.
	//
	// @param sink The object to notify when the read has completed,
	//            successfully or otherwise.
	// @param buffer The buffer to copy the read bytes into.
	Read(self UploadDataProvider, sink UploadDataSink, buffer Buffer)

	// Rewind
	// Rewinds upload data. Each call must be followed be a single
	// call, either synchronous or asynchronous, to
	// UploadDataSink.OnRewindSucceeded() on success or
	// UploadDataSink.OnRewindError() on failure. Neither read nor rewind
	// will be called until one of those methods or the other is called.
	// Even if the associated UrlRequest is canceled, one or the other
	// must still be called before resources can be safely freed.
	//
	// If rewinding is not supported, this should call
	// UploadDataSink.OnRewindError(). Note that rewinding is required to
	// follow redirects that preserve the upload body, and for retrying when the
	// server times out stale sockets.
	//
	// @param sink The object to notify when the rewind operation has
	//         completed, successfully or otherwise.
	Rewind(self UploadDataProvider, sink UploadDataSink)

	// Close
	// Called when this UploadDataProvider is no longer needed by a request, so that resources
	// (like a file) can be explicitly released.
	Close(self UploadDataProvider)
}
