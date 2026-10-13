# AEGIS Threat Model & Cryptographic Security Architecture

## 1. Executive Summary

Project AEGIS (National Digital Assessment Platform) is engineered specifically to eliminate **question paper leaks prior to examinations** and **post-exam result tampering**.

Traditional examination architectures suffer from a critical flaw: **Centralized Trust**. A single database administrator, cloud operator, or compromised executive with infrastructure credentials can dump the database or extract the master key days before the exam, resulting in catastrophic national paper leaks.

AEGIS addresses this by replacing single-authority trust with **Mathematical Threshold Custody (Shamir $k$-of-$n$)** and **Time-Locked Cryptographic Release**.

---

## 2. Threat Actor Taxonomy

| Threat Actor | Capabilities | Motivation | Primary Vector | AEGIS Mitigation |
|---|---|---|---|---|
| **Corrupt Database Admin (DBA)** | Root database access, volume snapshots, disk backups | Financial gain, collusion | Direct SQL dump of question tables | **Full Content Envelope Encryption (AES-256-GCM)**; items stored encrypted. DBA sees only ciphertext. |
| **Corrupt High Official / Insider** | Administrative UI access, policy controls | Nepotism, extortion | Premature paper decryption | **Threshold Key Custody (Shamir $k$-of-$n$)**; no single individual possesses the key. |
| **External Attacker / Cloud Intruder** | Network interception, cloud credential compromise | Espionage, sabotage | Man-in-the-middle, early API calls | **Time-Lock Release Guard**; paper is mathematically unreadable until timestamp $T$. |
| **Rogue Exam Centre Operator** | Physical access to testing terminals | Paper distribution | Screenshot, photographing screens | **Per-Candidate Steganographic Watermarking**; every question stem embeds candidate & centre tracking tokens. |
| **Candidate / Proxy Test-Taker** | Client device manipulation | Score inflation, repudiation | Modifying submitted answers post-hoc | **Client-Side Signature Capture + Server HMAC**; responses are immutably chained in Merkle audit logs. |

---

## 3. Core Cryptographic Defenses

### A. Threshold Key Custody (Shamir's Secret Sharing)
* **Algorithm**: Polynomial interpolation over Galois Field $\text{GF}(2^8)$ (Rijndael polynomial `0x11B`).
* **Mechanism**: Every generated exam paper is encrypted under a unique 256-bit symmetric AES key $K_{paper}$.
* **Distribution**: $K_{paper}$ is split into $N$ custodian shares with threshold $K$ (e.g., $K=3, N=5$ or $K=5, N=9$).
* **Custodians**: Held by independent legal institutions:
  1. Central Examination Board
  2. Judicial Oversight / High Court Nominee
  3. National Cyber Security Directorate
  4. Independent University Vice-Chancellor
  5. Accredited Technical Auditor
* **Mathematical Invariant**: Any $K-1$ or fewer colluding custodians possess **exactly zero mutual information** ($I(K_{paper}; \text{Shares}_{<K}) = 0$) regarding the paper encryption key.

### B. Time-Locked Release Guard ($T_{start}$)
* Custodian shares are submitted to the release guard at exam start time $T$.
* Even if all $N$ custodians submit their shares early, the time-lock guard mathematically rejects decryption before $T$.
* Automated tests prove early decryption is rejected (`TestPaperCannotBeDecryptedBeforeT`).

### C. Candidate Watermarking & Leak Traceability
* When questions are delivered to an in-progress session, the candidate's unique identity hash is embedded into the question content using steganographic metadata tokens:
  ```html
  Which fundamental right is guaranteed under Article 21?
  <!-- AEGIS-AUDIT:[cand-7a9f:item-4c12] -->
  ```
* If a photograph or digital copy of the screen is leaked on social media, the forensic token immediately pinpoints the exact candidate terminal and exam centre.

### D. Answer Integrity & Non-Repudiation
* Every submitted answer records:
  - `ClientHash`: Candidate's client-side cryptographic HMAC or digital signature.
  - `ServerHash`: Monotonic sequence-numbered server HMAC.
  - `ServerTimestamp`: NTP-synchronized authoritative timestamp.
* Responses are written to an append-only Merkle hash chain (`SHA-256(PreviousHash || Entry)`), making silent database modifications cryptographically detectable.

---

## 4. Verification & Automated Proof

The cryptographic claims of this threat model are continuously verified in CI via automated unit tests:
* `pkg/crypto/threshold/shamir_test.go`:
  - `TestShamirSplitAndCombine` (Reconstruction with $K$ shares)
  - `TestShamirKMinus1CannotReconstruct` (Proves $K-1$ shares fail)
  - `TestPaperCannotBeDecryptedBeforeT` (Proves early release is blocked)
  - `TestTamperedCiphertextDetected` (Proves payload tampering is caught)
* `pkg/crypto/persistent_km_test.go`:
  - `TestPersistentKeyManagerSurvivesRestarts` (Proves keys survive pod restarts)
* `internal/delivery/service_test.go`:
  - `TestStartSessionEnforcesReleaseTimeT` (Proves release time is enforced)
  - `TestGetQuestionsDeliversRedactedAndWatermarkedQuestions` (Proves watermarking)
