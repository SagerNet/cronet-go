// Copyright 2017 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

#ifndef COMPONENTS_CRONET_NATIVE_INCLUDE_CRONET_C_H_
#define COMPONENTS_CRONET_NATIVE_INCLUDE_CRONET_C_H_

#include "cronet_export.h"

// Cronet public C API is generated from cronet.idl
#include "cronet.idl_c.h"

#ifdef __cplusplus
extern "C" {
#endif

// Stream Engine used by Bidirectional Stream C API for GRPC.
typedef struct stream_engine stream_engine;

// Additional Cronet C API not generated from cronet.idl.

// Sets net::CertVerifier* raw_mock_cert_verifier for testing of Cronet_Engine.
// Must be called before Cronet_Engine_InitWithParams().
CRONET_EXPORT void Cronet_Engine_SetMockCertVerifierForTesting(
    Cronet_EnginePtr engine,
    /* net::CertVerifier* */ void* raw_mock_cert_verifier);

// Returns "stream_engine" interface for bidirectionsl stream support for GRPC.
// Returned stream engine is owned by Cronet Engine and is only valid until
// Cronet_Engine_Shutdown().
CRONET_EXPORT stream_engine* Cronet_Engine_GetStreamEngine(
    Cronet_EnginePtr engine);

// Creates a CertVerifier that uses custom root certificates for validation.
// pem_root_certs: PEM-formatted root certificates (can contain multiple certs).
// Returns a pointer to the created net::CertVerifier.
// The caller is responsible for passing it to
// Cronet_Engine_SetMockCertVerifierForTesting() which takes ownership.
// Returns nullptr if the PEM data is invalid or no valid certificates found.
CRONET_EXPORT void* Cronet_CreateCertVerifierWithRootCerts(
    const char* pem_root_certs);

// Dialer callback type for custom TCP connection establishment.
// context: User-provided context pointer passed to Cronet_Engine_SetDialer.
// address: IP address string (e.g. "1.2.3.4" or "::1").
// port: Port number.
// Returns: connected socket fd on success, negative net error code on failure.
// Common error codes:
//   -102: ERR_CONNECTION_REFUSED
//   -104: ERR_CONNECTION_FAILED
//   -109: ERR_ADDRESS_UNREACHABLE
//   -118: ERR_CONNECTION_TIMED_OUT
typedef int (*Cronet_DialerFunc)(void* context,
                                 const char* address,
                                 uint16_t port);

// Sets a custom dialer for TCP connections.
// When set, the engine will use this callback to establish TCP connections
// instead of the default system socket API.
// Must be called before Cronet_Engine_StartWithParams().
// dialer: The callback function to use for TCP connections, or nullptr to
//         disable custom dialing.
// context: User-provided context pointer that will be passed to the dialer.
CRONET_EXPORT void Cronet_Engine_SetDialer(Cronet_EnginePtr engine,
                                           Cronet_DialerFunc dialer,
                                           void* context);

// UDP Dialer callback type for custom UDP socket creation.
// context: User-provided context pointer passed to Cronet_Engine_SetUdpDialer.
// address: IP address string (e.g. "1.2.3.4" or "::1").
// port: Port number.
// out_local_address: Output buffer for local IP address (caller provides buffer,
//                    should be at least 46 bytes for INET6_ADDRSTRLEN).
// out_local_port: Output pointer for local port number.
// Returns: socket fd on success, negative net error code on failure.
// The returned socket can be:
//   - AF_INET/AF_INET6 SOCK_DGRAM: Standard UDP socket
//   - AF_UNIX SOCK_DGRAM: Unix domain datagram socket (Unix/macOS/Linux)
//   - AF_UNIX SOCK_STREAM: Unix domain stream socket (Windows, with framing)
typedef int (*Cronet_UdpDialerFunc)(void* context,
                                    const char* address,
                                    uint16_t port,
                                    char* out_local_address,
                                    uint16_t* out_local_port);

// Sets a custom dialer for UDP sockets.
// When set, the engine will use this callback to create UDP sockets
// instead of the default system socket API.
// Must be called before Cronet_Engine_StartWithParams().
// dialer: The callback function to use for UDP sockets, or nullptr to
//         disable custom UDP dialing.
// context: User-provided context pointer that will be passed to the dialer.
CRONET_EXPORT void Cronet_Engine_SetUdpDialer(Cronet_EnginePtr engine,
                                              Cronet_UdpDialerFunc dialer,
                                              void* context);

#ifdef __cplusplus
}
#endif

#endif  // COMPONENTS_CRONET_NATIVE_INCLUDE_CRONET_C_H_
