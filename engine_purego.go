//go:build with_purego

package cronet

import (
	"unsafe"

	"github.com/sagernet/cronet-go/internal/cronet"
)

func NewEngine() Engine {
	return Engine{cronet.EngineCreate()}
}

func (e Engine) Destroy() {
	cronet.EngineDestroy(e.ptr)
}

// StartWithParams starts Engine using given |params|. The engine must be started once
// and only once before other methods can be used.
func (e Engine) StartWithParams(params EngineParams) Result {
	return Result(cronet.EngineStartWithParams(e.ptr, params.ptr))
}

// StartNetLogToFile starts NetLog logging to a file. The NetLog will contain events emitted
// by all live Engines. The NetLog is useful for debugging.
// The file can be viewed using a Chrome browser navigated to
// chrome://net-internals/#import
// Returns |true| if netlog has started successfully, |false| otherwise.
// @param fileName the complete file path. It must not be empty. If the file
//
//	exists, it is truncated before starting. If actively logging,
//	this method is ignored.
//
// @param logAll to include basic events, user cookies,
//
//	credentials and all transferred bytes in the log. This option presents
//	a privacy risk, since it exposes the user's credentials, and should
//	only be used with the user's consent and in situations where the log
//	won't be public. false to just include basic events.
func (e Engine) StartNetLogToFile(fileName string, logAll bool) bool {
	return cronet.EngineStartNetLogToFile(e.ptr, fileName, logAll)
}

// StopNetLog Stops NetLog logging and flushes file to disk. If a logging session is
// not in progress, this call is ignored. This method blocks until the log is
// closed to ensure that log file is complete and available.
func (e Engine) StopNetLog() {
	cronet.EngineStopNetLog(e.ptr)
}

// Shutdown shuts down the Engine if there are no active requests,
// otherwise returns a failure Result.
//
// Cannot be called on network thread - the thread Cronet calls into
// Executor on (which is different from the thread the Executor invokes
// callbacks on). This method blocks until all the Engine's resources have
// been cleaned up.
func (e Engine) Shutdown() Result {
	return Result(cronet.EngineShutdown(e.ptr))
}

// Version returns a human-readable version string of the engine.
func (e Engine) Version() string {
	return cronet.EngineGetVersionString(e.ptr)
}

// DefaultUserAgent Returns default human-readable version string of the engine. Can be used
// before StartWithParams() is called.
func (e Engine) DefaultUserAgent() string {
	return cronet.EngineGetDefaultUserAgent(e.ptr)
}

// AddRequestFinishListener registers a listener that gets called at the end of each request.
//
// The listener is called on Executor.
//
// The listener is called before URLRequestCallbackHandler.OnCanceled(),
// URLRequestCallbackHandler.OnFailed() or
// URLRequestCallbackHandler.OnSucceeded() is called -- note that if Executor
// runs the listener asynchronously, the actual call to the listener
// may happen after a URLRequestCallbackHandler method is called.
//
// Listeners are only guaranteed to be called for requests that are started
// after the listener is added.
//
// Ownership is **not** taken for listener or Executor.
//
// Assuming the listener won't run again (there are no pending requests with
// the listener attached, either via Engine or UrlRequest),
// the app may destroy it once its OnRequestFinished() has started,
// even inside that method.
//
// Similarly, the app may destroy executor in or after OnRequestFinished()}.
//
// It's also OK to destroy executor in or after one of
// URLRequestCallbackHandler.OnCanceled(), URLRequestCallbackHandler.OnFailed() or
// URLRequestCallbackHandler.OnSucceeded().
//
// Of course, both of these are only true if listener won't run again
// and executor isn't being used for anything else that might start
// running in the future.
//
// @param listener the listener for finished requests.
// @param executor the executor upon which to run listener.
func (e Engine) AddRequestFinishListener(listener URLRequestFinishedInfoListener, executor Executor) {
	cronet.EngineAddRequestFinishedListener(e.ptr, listener.ptr, executor.ptr)
}

// RemoveRequestFinishListener unregisters a RequestFinishedInfoListener,
// including its association with its registered Executor.
func (e Engine) RemoveRequestFinishListener(listener URLRequestFinishedInfoListener) {
	cronet.EngineRemoveRequestFinishedListener(e.ptr, listener.ptr)
}

func (e Engine) SetClientContext(context unsafe.Pointer) {
	cronet.EngineSetClientContext(e.ptr, uintptr(context))
}

func (e Engine) ClientContext() unsafe.Pointer {
	return unsafe.Pointer(cronet.EngineGetClientContext(e.ptr))
}

// SetTrustedRootCertificates sets custom trusted root certificates for this engine.
// Must be called before StartWithParams().
// pemRootCerts should be PEM-formatted certificates (can contain multiple certificates).
// Returns true if the certificates were successfully set, false if parsing failed.
func (e Engine) SetTrustedRootCertificates(pemRootCerts string) bool {
	certVerifier := cronet.CreateCertVerifierWithRootCerts(pemRootCerts)
	if certVerifier == 0 {
		return false
	}
	cronet.EngineSetMockCertVerifierForTesting(e.ptr, certVerifier)
	return true
}

// SetCertVerifierWithPublicKeySHA256 sets a certificate verifier that validates
// certificates by matching the public key SHA256 hash, bypassing CA chain validation.
// This is similar to sing-box's certificate_public_key_sha256 behavior.
// Must be called before StartWithParams().
// hashes should be raw 32-byte SHA256 hashes of the certificate's SPKI.
// Returns true if the verifier was successfully set, false if no hashes provided.
func (e Engine) SetCertVerifierWithPublicKeySHA256(hashes [][]byte) bool {
	if len(hashes) == 0 {
		return false
	}
	for _, hash := range hashes {
		if len(hash) != 32 {
			return false
		}
	}
	certVerifier := cronet.CreateCertVerifierWithPublicKeySHA256(hashes)
	if certVerifier == 0 {
		return false
	}
	cronet.EngineSetMockCertVerifierForTesting(e.ptr, certVerifier)
	return true
}
