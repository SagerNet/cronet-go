package cronet

// #include <stdlib.h>
// #include <stdbool.h>
// #include <cronet_c.h>
import "C"

import (
	"unsafe"
)

type EngineParameters struct {
	ptr C.Cronet_EngineParamsPtr
}

func NewEngineParameters() EngineParameters {
	return EngineParameters{C.Cronet_EngineParams_Create()}
}

func (p EngineParameters) Destroy() {
	C.Cronet_EngineParams_Destroy(p.ptr)
}

func (p EngineParameters) EnableCheckResult() bool {
	return bool(C.Cronet_EngineParams_enable_check_result_get(p.ptr))
}

func (p EngineParameters) SetEnableCheckResult(enable bool) {
	C.Cronet_EngineParams_enable_check_result_set(p.ptr, C.bool(enable))
}

func (p EngineParameters) UserAgent() string {
	return C.GoString(C.Cronet_EngineParams_user_agent_get(p.ptr))
}

func (p EngineParameters) SetUserAgent(userAgent string) {
	cUserAgent := C.CString(userAgent)
	C.Cronet_EngineParams_user_agent_set(p.ptr, cUserAgent)
	C.free(unsafe.Pointer(cUserAgent))
}

func (p EngineParameters) AccentLanguage() string {
	return C.GoString(C.Cronet_EngineParams_accept_language_get(p.ptr))
}

func (p EngineParameters) SetAccentLanguage(acceptLanguage string) {
	cAcceptLanguage := C.CString(acceptLanguage)
	C.Cronet_EngineParams_accept_language_set(p.ptr, cAcceptLanguage)
	C.free(unsafe.Pointer(cAcceptLanguage))
}

func (p EngineParameters) StoragePath() string {
	return C.GoString(C.Cronet_EngineParams_storage_path_get(p.ptr))
}

func (p EngineParameters) SetStoragePath(storagePath string) {
	cStoragePath := C.CString(storagePath)
	C.Cronet_EngineParams_storage_path_set(p.ptr, cStoragePath)
	C.free(unsafe.Pointer(cStoragePath))
}

func (p EngineParameters) EnableQuic() bool {
	return bool(C.Cronet_EngineParams_enable_quic_get(p.ptr))
}

func (p EngineParameters) SetEnableQuic(enable bool) {
	C.Cronet_EngineParams_enable_quic_set(p.ptr, C.bool(enable))
}

func (p EngineParameters) EnableHTTP2() bool {
	return bool(C.Cronet_EngineParams_enable_http2_get(p.ptr))
}

func (p EngineParameters) SetEnableHTTP2(enable bool) {
	C.Cronet_EngineParams_enable_http2_set(p.ptr, C.bool(enable))
}

func (p EngineParameters) EnableBrotli() bool {
	return bool(C.Cronet_EngineParams_enable_brotli_get(p.ptr))
}

func (p EngineParameters) SetEnableBrotli(enable bool) {
	C.Cronet_EngineParams_enable_brotli_set(p.ptr, C.bool(enable))
}

type HTTPCacheMode uint8

const (
	HTTPCacheModeDisabled HTTPCacheMode = iota
	HTTPCacheModeInMemory
	HTTPCacheModeDiskNoHTTP
	HTTPCacheModeDisk
)

func (p EngineParameters) HTTPCacheMode() HTTPCacheMode {
	return HTTPCacheMode(C.Cronet_EngineParams_http_cache_mode_get(p.ptr))
}

func (p EngineParameters) SetHTTPCacheMode(mode HTTPCacheMode) {
	C.Cronet_EngineParams_http_cache_mode_set(p.ptr, C.Cronet_EngineParams_HTTP_CACHE_MODE(mode))
}

func (p EngineParameters) HTTPCacheMaxSize() int64 {
	return int64(C.Cronet_EngineParams_http_cache_max_size_get(p.ptr))
}

func (p EngineParameters) SetHTTPCacheMaxSize(maxSize int64) {
	C.Cronet_EngineParams_http_cache_max_size_set(p.ptr, C.int64_t(maxSize))
}

func (p EngineParameters) AddQuicHints(element QuicHint) {
	C.Cronet_EngineParams_quic_hints_add(p.ptr, element.ptr)
}

func (p EngineParameters) QuicHintsSize() int {
	return int(C.Cronet_EngineParams_quic_hints_size(p.ptr))
}

func (p EngineParameters) QuicHintsAt(index int) *QuicHint {
	ptr := C.Cronet_EngineParams_quic_hints_at(p.ptr, C.uint32_t(index))
	if ptr == nil {
		return nil
	}
	return &QuicHint{ptr}
}

func (p EngineParameters) ClearQuicHints() {
	C.Cronet_EngineParams_quic_hints_clear(p.ptr)
}

func (p EngineParameters) AddPublicKeyPins(element PublicKeyPins) {
	C.Cronet_EngineParams_public_key_pins_add(p.ptr, element.ptr)
}

func (p EngineParameters) PublicKeyPinsSize() int {
	return int(C.Cronet_EngineParams_public_key_pins_size(p.ptr))
}

func (p EngineParameters) PublicKeyPinsAt(index int) *PublicKeyPins {
	ptr := C.Cronet_EngineParams_public_key_pins_at(p.ptr, C.uint32_t(index))
	if ptr == nil {
		return nil
	}
	return &PublicKeyPins{ptr}
}

func (p EngineParameters) ClearPublicKeyPins() {
	C.Cronet_EngineParams_public_key_pins_clear(p.ptr)
}

func (p EngineParameters) EnablePublicKeyPinningBypassForLocalTrustAnchors() bool {
	return bool(C.Cronet_EngineParams_enable_public_key_pinning_bypass_for_local_trust_anchors_get(p.ptr))
}

func (p EngineParameters) SetEnablePublicKeyPinningBypassForLocalTrustAnchors(enable bool) {
	C.Cronet_EngineParams_enable_public_key_pinning_bypass_for_local_trust_anchors_set(p.ptr, C.bool(enable))
}

func (p EngineParameters) NetworkThreadPriority() int {
	return int(C.Cronet_EngineParams_network_thread_priority_get(p.ptr))
}

func (p EngineParameters) SetNetworkThreadPriority(priority int) {
	C.Cronet_EngineParams_network_thread_priority_set(p.ptr, C.double(priority))
}

func (p EngineParameters) ExperimentalOptions() string {
	return C.GoString(C.Cronet_EngineParams_experimental_options_get(p.ptr))
}

func (p EngineParameters) SetExperimentalOptions(options string) {
	cOptions := C.CString(options)
	C.Cronet_EngineParams_experimental_options_set(p.ptr, cOptions)
	C.free(unsafe.Pointer(cOptions))
}
