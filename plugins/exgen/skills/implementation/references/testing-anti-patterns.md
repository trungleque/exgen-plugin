# Testing Anti-Patterns

**Load this reference when:** introducing mocks, stubs, spies, or test doubles, or if tempted to add test-only methods to production code.

**Core Principle:** Test what the code does, not what mocks do. Mocks isolate slow or external systems; they are never the subject under test. Following strict TDD prevents these anti-patterns.

---

## The Iron Laws

1. **NEVER test mock behavior.** Assert on real component outputs, never on mock presence or call counts of internal plumbing.
2. **NEVER add test-only methods to production classes.** Lifecycle cleanup belongs in test utilities.
3. **NEVER mock without understanding dependencies.** Preserve side effects the test relies upon.
4. **NEVER create partial mocks.** Mirror the complete data structure of real APIs/entities.

---

## 1. Testing Mock Behavior

❌ **Bad (asserting on mock presence):**
```typescript
render(<Page />);
expect(screen.getByTestId('sidebar-mock')).toBeInTheDocument();
```
✅ **Good (test real component or unmock):**
```typescript
render(<Page />); // Don't mock Sidebar, or if isolated, test Page's observable behavior:
expect(screen.getByRole('navigation')).toBeInTheDocument();
```
**Gate Function:** Before asserting on any mock element:
- Ask: *"Am I testing real behavior or mock existence?"*
- If mock existence: **STOP.** Delete the assertion or unmock the dependency.

---

## 2. Test-Only Methods in Production

❌ **Bad (production API polluted for test cleanup):**
```typescript
class Session {
  async destroy() { await this.manager.destroy(this.id); } // Only used in tests
}
afterEach(() => session.destroy());
```
✅ **Good (test utilities manage test lifecycles):**
```typescript
// in test-utils/session.ts
export async function cleanupSession(s: Session) { await manager.destroy(s.id); }
afterEach(() => cleanupSession(session));
```
**Gate Function:** Before adding a method to a production class:
- Ask: *"Is this only used by tests?"* If yes, **STOP.** Put it in test utilities.
- Ask: *"Does this class own this resource's lifecycle?"* If no, **STOP.**

---

## 3. Mocking Without Understanding

❌ **Bad (mocking breaks side effects needed by the test):**
```typescript
// discoverAndCacheTools wrote config; mocking it breaks duplicate server detection!
vi.mock('ToolCatalog', () => ({ discoverAndCacheTools: vi.fn() }));
await addServer(cfg);
await addServer(cfg); // Should throw duplicate error, but won't!
```
✅ **Good (mock at the lowest isolation boundary):**
```typescript
// Mock only the slow server startup, preserving the config write behavior
vi.mock('MCPServerManager');
await addServer(cfg);
await addServer(cfg); // Duplicate detected ✓
```
**Gate Function:** Before mocking any method:
1. What side effects does the real method produce?
2. Does this test depend on any of those side effects? If yes, mock at a lower level or use a state-preserving double.
3. If unsure, run with the real implementation first, observe dependencies, then isolate minimally.

---

## 4. Incomplete Mocks

❌ **Bad (partial mock omitting fields downstream code expects):**
```typescript
const mockResponse = { status: 'success', data: { userId: '123' } };
// Downstream breaks when reading response.metadata.requestId
```
✅ **Good (mirror real data structures completely):**
```typescript
const mockResponse = {
  status: 'success',
  data: { userId: '123', name: 'Alice' },
  metadata: { requestId: 'req-789', timestamp: 1234567890 }
};
```
**Gate Function:** Before creating mock responses:
- Inspect real schemas/docs. Include **all** fields the system might consume downstream. Never provide partial shapes.

---

## 5. Integration Tests as an Afterthought

❌ "Implementation complete, ready for testing." Tests *are* the implementation.
✅ Strict TDD: **Red** (failing test) → **Green** (minimal code) → **Refactor**. If you never saw the test fail against missing behavior, it proves nothing.

---

## Red Flags & Quick Reference

| Anti-Pattern | Red Flag | Corrective Action |
| :--- | :--- | :--- |
| **Testing mock behavior** | Assertion checks `*-mock` test IDs or internal mock calls | Test real behavior or unmock |
| **Test-only methods** | Methods only called in test files | Move helper to test utilities |
| **Unsound mocking** | Mocking "just to be safe"; setup is >50% of test | Run with real code first; mock lowest boundary |
| **Incomplete mocks** | Mock payload omits schema fields | Mirror full API response structure |
| **Over-complex mocks** | Mock setup longer than test logic | Replace with integration test using real components |
