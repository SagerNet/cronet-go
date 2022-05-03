package cronet

// #cgo CFLAGS: -I.
// #cgo LDFLAGS: -L. -lcronet.100.0.4896.60
// #include <stdlib.h>
// #include <stdbool.h>
// #include <cronet_c.h>
import "C"
import "unsafe"

type EngineParameters struct {
	ptr C.Cronet_EngineParamsPtr
}

func NewEngineParameters() *EngineParameters {
	return &EngineParameters{C.Cronet_EngineParams_Create()}
}

func (p *EngineParameters) Destroy() {
	C.Cronet_EngineParams_Destroy(p.ptr)
}

func (p *EngineParameters) SetEnableCheckResult(enable bool) {
	C.Cronet_EngineParams_enable_check_result_set(p.ptr, C.bool(enable))
}

func (p *EngineParameters) SetUserAgent(userAgent string) {
	cUserAgent := C.CString(userAgent)
	C.Cronet_EngineParams_user_agent_set(p.ptr, cUserAgent)
	C.free(unsafe.Pointer(cUserAgent))
}

func (p *EngineParameters) SetAccentLanguage(acceptLanguage string) {
	cAcceptLanguage := C.CString(acceptLanguage)
	C.Cronet_EngineParams_accept_language_set(p.ptr, cAcceptLanguage)
	C.free(unsafe.Pointer(cAcceptLanguage))
}

func (p *EngineParameters) SetStoragePath(storagePath string) {
	cStoragePath := C.CString(storagePath)
	C.Cronet_EngineParams_storage_path_set(p.ptr, cStoragePath)
	C.free(unsafe.Pointer(cStoragePath))
}

func (p *EngineParameters) SetEnableQuic(enable bool) {
	C.Cronet_EngineParams_enable_quic_set(p.ptr, C.bool(enable))
}

func (p *EngineParameters) SetEnableHTTP2(enable bool) {
	C.Cronet_EngineParams_enable_http2_set(p.ptr, C.bool(enable))
}

func (p *EngineParameters) SetEnableBrotli(enable bool) {
	C.Cronet_EngineParams_enable_brotli_set(p.ptr, C.bool(enable))
}

type HTTPCacheMode uint8

const (
	HTTPCacheModeDisabled HTTPCacheMode = iota
	HTTPCacheModeInMemory
	HTTPCacheModeDiskNoHTTP
	HTTPCacheModeDisk
)

func (p *EngineParameters) SetHTTPCacheMode(mode HTTPCacheMode) {
	C.Cronet_EngineParams_http_cache_mode_set(p.ptr, C.Cronet_EngineParams_HTTP_CACHE_MODE(mode))
}

func (p *EngineParameters) SetHTTPCacheMaxSize(maxSize int64) {
	C.Cronet_EngineParams_http_cache_max_size_set(p.ptr, C.int64_t(maxSize))
}

func (p *EngineParameters) AddQuicHints(element *QuicHint) {
	C.Cronet_EngineParams_quic_hints_add(p.ptr, element.ptr)
}

func (p *EngineParameters) AddPublicKeyPins(element *PublicKeyPins) {
	C.Cronet_EngineParams_public_key_pins_add(p.ptr, element.ptr)
}

func (p *EngineParameters) SetEnablePublicKeyPinningBypassForLocalTrustAnchors(enable bool) {
	C.Cronet_EngineParams_enable_public_key_pinning_bypass_for_local_trust_anchors_set(p.ptr, C.bool(enable))
}

func (p *EngineParameters) SetNetworkThreadPriority(priority int) {
	C.Cronet_EngineParams_network_thread_priority_set(p.ptr, C.double(priority))
}

// TODO: finish this

func (p *EngineParameters) SetExperimentalOptions(options string) {
	cOptions := C.CString(options)
	C.Cronet_EngineParams_experimental_options_set(p.ptr, cOptions)
	C.free(unsafe.Pointer(cOptions))
}
