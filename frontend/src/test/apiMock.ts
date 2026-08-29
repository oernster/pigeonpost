// apiMock builds a module mock for `../api` from the real module rather than from a hand-written list
// beside it.
//
// A hand-written mock is the wire stated twice with nothing comparing the two statements. A method the
// app calls that the mock never declared is undefined, so calling it throws a TypeError; nearly every
// api call site sits inside a try/catch that turns a failure into a message in the error bar. The
// test then passes while the behaviour under test never ran. Five such holes were found one at a time,
// each by a new test that happened to reach one, which is not a way to find the rest.
//
// Here every method of the real api gets a stub. A declared spy is used as given, so tests configure it
// exactly as before. Anything else records that it was reached and throws, then the paired assertion
// fails the test naming the method, so a swallowed TypeError can no longer pass for working behaviour.

// ApiModule is the shape of the real module: the api object plus its value exports.
export interface ApiModule {
    api: Record<string, unknown>
}

// buildApiStubs returns the api object to mock with. declared holds the spies the test configures;
// reached is the set every unstubbed call records itself in.
export function buildApiStubs(
    actual: ApiModule,
    declared: Record<string, unknown>,
    reached: Set<string>,
): Record<string, unknown> {
    const stubs: Record<string, unknown> = {}
    for (const name of Object.keys(actual.api)) {
        stubs[name] = declared[name] ?? (() => {
            reached.add(name)
            throw new Error(`api.${name} has no stub in this test`)
        })
    }
    return stubs
}

// unstubbedNames drains the set and returns what it held, sorted. Call it from an afterEach and assert
// the result is empty; draining keeps one test's holes from failing the next.
export function unstubbedNames(reached: Set<string>): string[] {
    const names = [...reached].sort()
    reached.clear()
    return names
}

// spiesNotInApi returns the declared spy names the real api does not have. A spy under a name that no
// longer exists binds to nothing, so every test configuring it configures a stub the code can never
// call and passes for the wrong reason.
export function spiesNotInApi(actual: ApiModule, declared: Record<string, unknown>): string[] {
    const real = new Set(Object.keys(actual.api))
    return Object.keys(declared).filter((name) => !real.has(name))
}
