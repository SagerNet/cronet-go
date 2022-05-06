package cronet

// #include <stdlib.h>
// #include <stdbool.h>
// #include <cronet_c.h>
import "C"

// URLRequestCallback
// Users of Cronet implement this interface to receive callbacks indicating the
// progress of a URLRequest being processed. An instance of this interface
// is passed in to URLRequest.InitWithParams().
//
// Note:  All methods will be invoked on the Executor passed to URLRequest.InitWithParams();
type URLRequestCallback struct {
	ptr C.Cronet_UrlRequestCallbackPtr
}

type URLRequestCallbackHandler interface {
	// OnRedirectReceived
	// Invoked whenever a redirect is encountered. This will only be invoked
	// between the call to URLRequest.Start() and
	// URLRequestCallbackHandler.OnResponseStarted().
	// The body of the redirect response, if it has one, will be ignored.
	//
	// The redirect will not be followed until the URLRequest.FollowRedirect()
	// method is called, either synchronously or asynchronously.
	//
	// @param request Request being redirected.
	// @param info Response information.
	// @param newLocationUrl Location where request is redirected.
	OnRedirectReceived(self URLRequestCallback, request URLRequest, info URLResponseInfo, newLocationUrl string)

	// OnResponseStarted
	// Invoked when the final set of headers, after all redirects, is received.
	// Will only be invoked once for each request.
	//
	// With the exception of URLRequestCallbackHandler.OnCanceled(),
	// no other URLRequestCallbackHandler method will be invoked for the request,
	// including URLRequestCallbackHandler.OnSucceeded() and
	// URLRequestCallbackHandler.OnFailed(), until URLRequest.Read() is called to attempt
	// to start reading the response body.
	//
	// @param request Request that started to get response.
	// @param info Response information.
	OnResponseStarted(self URLRequestCallback, request URLRequest, info URLResponseInfo)

	// OnReadCompleted
	// Invoked whenever part of the response body has been read. Only part of
	// the buffer may be populated, even if the entire response body has not yet
	// been consumed. This callback transfers ownership of |buffer| back to the app,
	// and Cronet guarantees not to access it.
	//
	// With the exception of URLRequestCallbackHandler.OnCanceled(),
	// no other URLRequestCallbackHandler method will be invoked for the request,
	// including URLRequestCallbackHandler.OnSucceeded() and
	// URLRequestCallbackHandler.OnFailed(), until URLRequest.Read() is called to attempt
	// to continue reading the response body.
	//
	// @param request Request that received data.
	// @param info Response information.
	// @param buffer The buffer that was passed in to URLRequest.Read(), now
	//         containing the received data.
	// @param bytesRead The number of bytes read into buffer.
	OnReadCompleted(self URLRequestCallback, request URLRequest, info URLResponseInfo, buffer Buffer, bytesRead int64)

	// OnSucceeded
	// Invoked when request is completed successfully. Once invoked, no other
	// URLRequestCallbackHandler methods will be invoked.
	//
	// Implementations of OnSucceeded() are allowed to call
	// URLRequest.Destroy(), but note that destroying request destroys info.
	//
	// @param request Request that succeeded.
	// @param info Response information. NOTE: this is owned by request.
	OnSucceeded(self URLRequestCallback, request URLRequest, info URLResponseInfo)

	// OnFailed
	// Invoked if request failed for any reason after URLRequest.Start().
	// Once invoked, no other URLRequestCallbackHandler methods will be invoked.
	// |error| provides information about the failure.
	//
	// Implementations of URLRequestCallbackHandler.OnFailed are allowed to call
	// URLRequest.Destroy(), but note that destroying request destroys info and error.
	//
	// @param request Request that failed.
	// @param info Response information. May be null if no response was
	//         received. NOTE: this is owned by request.
	// @param error information about error. NOTE: this is owned by request
	OnFailed(self URLRequestCallback, request URLRequest, info URLResponseInfo, error Error)

	// OnCanceled
	// Invoked if request was canceled via URLRequest.Cancel(). Once
	// invoked, no other UrlRequestCallback methods will be invoked.
	//
	// Implementations of URLRequestCallbackHandler.OnCanceled are allowed to call
	// URLRequest.Destroy(), but note that destroying request destroys info and error.
	//
	// @param request Request that was canceled.
	// @param info Response information. May be null if no response was
	//         received. NOTE: this is owned by request.
	OnCanceled(self URLRequestCallback, request URLRequest, info URLResponseInfo)
}
