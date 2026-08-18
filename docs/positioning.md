# ngchain Positioning — frontier design, bounded by usability

ngchain's design rule is a single sentence: **adopt the most advanced
technique available, but only up to the point where it would cost
usability, determinism, or consensus safety.** The frontier is the goal;
usability is the constraint that shapes which part of the frontier we
actually ship.

This document explains where ngchain sits and why. For the concrete
mechanics see [implementation.md](./implementation.md); for the contract
lifecycle see [contract.md](./contract.md).

## Designed by subtraction

The whole ontology is **two nouns and six verbs** (see the README): an
Address is identity, balance and namespace at once; a Contract is the
code slot an address opens under itself. Nothing is registered, any key
spends directly, contracts are plain WebAssembly text edited by diff
hunks like a git repo.

Subtraction *is* the usability strategy. A smaller surface is easier to
reason about, harder to misuse, and cheaper to keep deterministic across
every node. Every feature below earns its place against this baseline —
it is added only when it buys real capability without re-expanding the
surface a user has to hold in their head.

## WebAssembly, not a bespoke VM

Contracts are WebAssembly, so any language that targets wasm (Rust,
AssemblyScript, TinyGo, C) produces a contract, with standard tooling
(`wasm-objdump`, standard compilers) and a sandbox with a decade of
hardening. That is the frontier choice *and* the usable one at the same
time — a rare alignment.

The discipline shows in what we **declined**. 256-bit integer math is
emulated on every non-EVM VM (wasm, RISC-V, eBPF all top out at 64-bit
arithmetic), and it is genuinely expensive in software. The maximal move
would be a custom wasm ISA extension with native `i256`, via a forked
LLVM backend. We rejected it: a forked toolchain is a permanent
maintenance tax and abandons "it's just wasm" (every external tool stops
understanding our modules). The benefit over a host function is a single
boundary crossing — not worth the cost. Frontier ambition, usability
veto.

## Determinism is the hard constraint

A consensus chain must compute the same result *and the same gas* on
every validator, on every architecture. This is the filter every
performance idea passes through, and it is why some frontier features
ship and others wait:

- **256-bit money — shipped, as a host module.** Token amounts are exact
  `U256` (an 18-decimal asset works precisely), but the hot operations
  (`mul_div`, `div`, `isqrt`) run in a native Go `wideint` host module
  over 32-byte little-endian operands — native speed, and deterministic
  because integer results are exact and platform-independent. A pure
  in-wasm bignum was correct but ~1000x slower on real DeFi calls (a
  liquidation burned a third of the per-call gas cap); the host module
  removed that ceiling without touching gas semantics.

- **Native execution (JIT) — shipped only once it was gas-identical.**
  wasman's baseline JIT compiles bodies to native code. We kept it
  **off** while its compiled code either skipped gas metering or
  miscompiled contracts — enabling it then would have split consensus.
  We turned it **on** only after an inline-metered JIT charged the *same*
  per-op toll as the interpreter: verified byte-identical gas across
  every scenario, JIT on vs off. So an arm64 node (JIT) and an x86 node
  (metered interpreter) agree exactly. Speed on the frontier, gas on the
  rails.

- **Module cache — shipped, because it changes nothing observable.**
  Contracts are decoded and validated once (keyed by code hash) instead
  of on every call. Pure wall-clock; zero effect on results, gas, or
  determinism, so it needed no such gate.

## Native, not imitated

Where a wasm-native mechanism exists, ngchain uses it instead of porting
an EVM-ism. Calls dispatch by the **export name** a wasm module already
carries — no eth-style 4-byte keccak selector, and therefore no selector
collision class to guard. The token model is **transfer-in** (send, then
call, settle on the balance delta) rather than allowance/`transfer_from`,
which removes a whole approval-exploit surface.

## Where this puts ngchain

| Axis | ngchain |
|---|---|
| Consensus | proof-of-work |
| State ontology | 2 nouns, 6 verbs — minimal by construction |
| Contract format | WebAssembly (any source language), stored as editable wat |
| Contract upgrade | diff-hunk patches on the text, like a git repo |
| Integers | exact 256-bit via a deterministic host module (no EVM 256-bit-word tax, no forked ISA) |
| Execution | metered JIT with gas byte-identical to the interpreter |
| Call dispatch | by wasm export name (no selectors) |
| Token model | transfer-in settlement (no allowance) |

The through-line: take the newest thing that survives the determinism and
usability filters, and leave the rest on the table — visibly, on purpose.
