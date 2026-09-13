# PulsarFi Project Rules & Architecture Constraints

These rules are strict, non-negotiable, and apply to all agents, subagents, and tools operating within this repository.

---

## 1. Directory Infrastructure & Zero-Tolerance Boundary Policy

Never violate the repository directory structure or place files in unauthorized directories.

### Backend (`backend/`)
- **`backend/src/`**: STRICTLY PRODUCTION CODE ONLY.
  - **DILARANG KERAS** meletakkan file testing (`*_test.go`), script sementara, scratch test, atau mock internal di dalam `backend/src/` (termasuk `backend/src/service/...`, `backend/src/handler/...`, `backend/src/repository/...`, dll).
  - Tidak ada pengecualian untuk alasan apa pun.
- **`backend/test/`**: SEMUA file automated testing, unit test, live integration test, dan benchmark Go HARUS berada di dalam `backend/test/...` (misalnya: `backend/test/service/agent/...`).

### Frontend (`frontend/`)
- **`frontend/components/`**, **`frontend/app/`**, **`frontend/http/`**: Komponen, hooks, API client, dan halaman aplikasi produksi.
- File testing React/Next.js (jika ada) hanya boleh berada di `frontend/__tests__/`.

### Smart Contract (`smart-contract/`)
- **`smart-contract/src/`**: Source code kontrak Solidity produksi saja.
- **`smart-contract/test/`**: SEMUA Foundry test file (`*.t.sol`) HARUS berada di `smart-contract/test/`.

### Documentation & Workplanning (`docs/`)
- **`docs/plans/`**: Rencana kerja, spesifikasi arsitektur, dan log perubahan teknis.

---

## 2. Strict Workplanning Sequence: Docs -> Implementation

**URUTAN TIDAK BOLEH DIOTAK-ATIK:**
1. **Update Docs:** Setiap perubahan fitur, perbaikan bug, atau penambahan logic wajib didokumentasikan dan di-update terlebih dahulu pada dokumen kerja terkait di `docs/plans/` (termasuk menaikkan versi dan mencatat changelog).
2. **Implementation:** Baru setelah dokumentasi diperbarui, implementasi kode pada production file boleh dilakukan.
3. Dilarang menyentuh kode sebelum docs diperbarui.

---

## 3. Frontend Contract Simulation (`simulateContract`)

- Setiap pemanggilan kontrak on-chain dari frontend (seperti ERC-20 `approve`, swap, atau deposit) **WAJIB** disimulasikan terlebih dahulu menggunakan:
  ```typescript
  const { request } = await publicClient.simulateContract({ ... });
  await writeContractAsync(request);
  ```
- Dilarang memanggil `writeContractAsync` mentah tanpa simulasi pre-flight `publicClient.simulateContract`.
- Tangkap dan format error pada level simulasi sebelum memunculkan popup wallet (MetaMask) ke user.

---

## 4. Strict Language Consistency & Global Error Policy

- **Agentic Chat Interaction:** Konsisten 100% mengikuti **Bahasa Pengguna (User's Language)** dalam seluruh interaksi percakapan chat, kartu konfirmasi UI, respon error agentic di chat, status task/subtask reasoning, dan chat prose.
  - Jika pengguna berinteraksi dalam **Bahasa Indonesia** -> 100% konsisten Bahasa Indonesia.
  - Jika pengguna berinteraksi dalam **Bahasa Inggris** -> 100% konsisten Bahasa Inggris.
  - Jika pengguna berinteraksi dalam **Bahasa Jepang** -> 100% konsisten Bahasa Jepang.
  - Dan seterusnya untuk bahasa apa pun yang digunakan pengguna.
- **System & Non-Agentic Errors:** **ERROR SELAIN AGENTIC ERROR DI CHAT WAJIB ENGLISH SEMUA!** Semua error smart contract, swap hooks error, transaction toast, wallet errors, dan modal UI non-chat WAJIB dalam Bahasa Inggris.
- **DILARANG KERAS** memaksakan satu bahasa tertentu atau mencampuradukkan bahasa (language mixing) di luar batasan di atas.
