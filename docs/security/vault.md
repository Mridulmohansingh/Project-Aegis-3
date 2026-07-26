# Cryptographic Secret & Envelope Encryption Design

This document details the secure storage logic for high-stakes test answers and solutions within AEGIS.

## Envelope Encryption Flow

We protect sensitive fields (MCQ answer keys, integer answers, and solutions) using the **envelope encryption pattern** via AES-256-GCM.

```
1. Encryption Request (Plaintext + Item ID)
       │
       ▼
2. Call KeyManager.GenerateDataKey(item_id)
       │
       ├──► Returns Plaintext DEK (in memory only)
       └──► Returns Encrypted DEK (wrapped with HSM Master KEK)
       │
       ▼
3. Encrypt Plaintext using AES-256-GCM
       │
       ├──► Nonce: 12-byte cryptographically secure random
       └──► Additional Authenticated Data (AAD): Item ID (prevents transplant)
       │
       ▼
4. Return EncryptedBlob (Ciphertext + Encrypted DEK)
```

## Key Management & HSM Integration

* **KeyManager Interface**: Located in `pkg/crypto`. It abstracts data key generation and unwrapping.
* **HSM/Vault Backend**: In production, the implementation connects via gRPC to HashiCorp Vault or cloud KMS (AWS KMS, GCP KMS). In development, it uses an in-memory key provider.
* **Zeroing Memory**: Plaintext keys (`dekPlaintext`) are cleared from memory using custom zero-bytes helpers immediately after use, preventing memory scrapers from extracting key materials.
