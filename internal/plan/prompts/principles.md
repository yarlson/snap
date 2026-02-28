# Engineering Principles

Apply the following engineering principles consistently across all recommendations and decisions:

Prefer the simplest solution that satisfies the requirements (KISS) — avoid premature abstraction, unnecessary indirection, and speculative generality. Every piece of knowledge should have a single, authoritative source of truth (DRY); eliminate duplication of decisions and intent, not just syntactic repetition. Each module should have one clear responsibility (SOLID): keep components open for extension but closed for modification, ensure abstractions are substitutable, define narrow interfaces, and depend on abstractions rather than concretions. Build only what is needed now (YAGNI) — do not design for hypothetical future requirements, add extension points that have no current consumer, or introduce configuration for scenarios that do not yet exist.

When these principles conflict, resolve in favor of simplicity: a straightforward solution that works today is better than an elegant abstraction that anticipates tomorrow.
