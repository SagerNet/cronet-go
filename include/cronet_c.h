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

// Creates a CertVerifier that validates certificates by matching the public key
// SHA256 hash, bypassing CA chain validation. This is similar to sing-box's
// certificate_public_key_sha256 behavior.
// hashes: Array of pointers to 32-byte SHA256 hashes (raw binary, not base64).
// hash_count: Number of hashes in the array.
// Returns a pointer to the created net::CertVerifier.
// The caller is responsible for passing it to
// Cronet_Engine_SetMockCertVerifierForTesting() which takes ownership.
// Returns nullptr if no hashes provided or invalid input.
CRONET_EXPORT void* Cronet_CreateCertVerifierWithPublicKeySHA256(
    const uint8_t** hashes,
    size_t hash_count);

#ifdef __cplusplus
}
#endif

#endif  // COMPONENTS_CRONET_NATIVE_INCLUDE_CRONET_C_H_
