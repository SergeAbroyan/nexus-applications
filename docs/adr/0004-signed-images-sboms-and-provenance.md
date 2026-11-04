# 0004. Every image is signed and carries an SBOM

Status: accepted

## Context

A registry accepts any image pushed with valid credentials, and a vulnerability
report needs to know what an image contains.

## Decision

CI signs each image digest with keyless cosign, using the GitHub OIDC identity
of the `ci.yaml` workflow on `main`, attaches an SPDX SBOM from Syft as a
signed attestation, and, when the repository is public, a SLSA build
provenance attestation from GitHub. Kyverno in the clusters admits only Nexus
images with that signature.

## Consequences

There are no signing keys to protect or rotate. An image built anywhere else
cannot run, so hotfixes must also go through CI.
