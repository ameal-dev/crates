package data

import "github.com/ameal-dev/crates/internal/db/models"

// ptr returns a pointer to a string.
func ptr(s string) *string { return &s }

// Curriculum is the full topic tree for all supported languages/frameworks.
var Curriculum = []models.Topic{
	{
		ID: "go", Slug: "go", Title: "Go", Description: "The Go programming language", SortOrder: 1,
		Children: []models.Topic{
			{
				ID: "go-basics", ParentID: ptr("go"), Slug: "go-basics", Title: "Basics", Description: "Go fundamentals", SortOrder: 1,
				Children: []models.Topic{
					{ID: "go-variables", ParentID: ptr("go-basics"), Slug: "go-variables", Title: "Variables & Types", Description: "var, :=, basic types, zero values, type inference", SortOrder: 1},
					{ID: "go-control-flow", ParentID: ptr("go-basics"), Slug: "go-control-flow", Title: "Control Flow", Description: "if/else, for loops, switch statements, defer", SortOrder: 2},
					{ID: "go-functions", ParentID: ptr("go-basics"), Slug: "go-functions", Title: "Functions", Description: "Multiple returns, named returns, variadic functions, closures", SortOrder: 3},
					{ID: "go-packages", ParentID: ptr("go-basics"), Slug: "go-packages", Title: "Packages & Imports", Description: "Package organization, visibility, go mod, internal packages", SortOrder: 4},
				},
			},
			{
				ID: "go-data-structures", ParentID: ptr("go"), Slug: "go-data-structures", Title: "Data Structures", Description: "Core Go data structures", SortOrder: 2,
				Children: []models.Topic{
					{ID: "go-arrays-slices", ParentID: ptr("go-data-structures"), Slug: "go-arrays-slices", Title: "Arrays & Slices", Description: "Arrays, slices, append, copy, slice internals", SortOrder: 1},
					{ID: "go-maps", ParentID: ptr("go-data-structures"), Slug: "go-maps", Title: "Maps", Description: "Map creation, access, iteration, comma-ok pattern", SortOrder: 2},
					{ID: "go-structs", ParentID: ptr("go-data-structures"), Slug: "go-structs", Title: "Structs", Description: "Struct definition, methods, embedding, tags", SortOrder: 3},
					{ID: "go-pointers", ParentID: ptr("go-data-structures"), Slug: "go-pointers", Title: "Pointers", Description: "Pointer basics, nil, pass by reference vs value", SortOrder: 4},
				},
			},
			{
				ID: "go-interfaces", ParentID: ptr("go"), Slug: "go-interfaces", Title: "Interfaces", Description: "Interface-based design in Go", SortOrder: 3,
				Children: []models.Topic{
					{ID: "go-interface-basics", ParentID: ptr("go-interfaces"), Slug: "go-interface-basics", Title: "Interface Basics", Description: "Implicit satisfaction, empty interface, type assertions", SortOrder: 1},
					{ID: "go-io-interfaces", ParentID: ptr("go-interfaces"), Slug: "go-io-interfaces", Title: "io.Reader & io.Writer", Description: "Standard library I/O interfaces, composition", SortOrder: 2},
					{ID: "go-interface-design", ParentID: ptr("go-interfaces"), Slug: "go-interface-design", Title: "Interface Design", Description: "Small interfaces, accept interfaces return structs", SortOrder: 3},
				},
			},
			{
				ID: "go-concurrency", ParentID: ptr("go"), Slug: "go-concurrency", Title: "Concurrency", Description: "Go's concurrency model", SortOrder: 4,
				Children: []models.Topic{
					{ID: "go-goroutines", ParentID: ptr("go-concurrency"), Slug: "go-goroutines", Title: "Goroutines", Description: "Launching goroutines, lifecycle, WaitGroup", SortOrder: 1},
					{ID: "go-channels", ParentID: ptr("go-concurrency"), Slug: "go-channels", Title: "Channels", Description: "Buffered/unbuffered channels, range, close", SortOrder: 2},
					{ID: "go-select", ParentID: ptr("go-concurrency"), Slug: "go-select", Title: "Select", Description: "Select statement, timeouts, non-blocking ops", SortOrder: 3},
					{ID: "go-context", ParentID: ptr("go-concurrency"), Slug: "go-context", Title: "Context", Description: "context.Context, cancellation, timeouts, values", SortOrder: 4},
					{ID: "go-sync", ParentID: ptr("go-concurrency"), Slug: "go-sync", Title: "Sync Package", Description: "Mutex, RWMutex, Once, Pool, atomic operations", SortOrder: 5},
				},
			},
			{
				ID: "go-error-handling", ParentID: ptr("go"), Slug: "go-error-handling", Title: "Error Handling", Description: "Go error patterns", SortOrder: 5,
				Children: []models.Topic{
					{ID: "go-errors-basics", ParentID: ptr("go-error-handling"), Slug: "go-errors-basics", Title: "Error Basics", Description: "error interface, fmt.Errorf, %w wrapping", SortOrder: 1},
					{ID: "go-custom-errors", ParentID: ptr("go-error-handling"), Slug: "go-custom-errors", Title: "Custom Errors", Description: "errors.Is, errors.As, sentinel errors, error types", SortOrder: 2},
				},
			},
			{
				ID: "go-generics", ParentID: ptr("go"), Slug: "go-generics", Title: "Generics", Description: "Go generics (1.18+)", SortOrder: 6,
				Children: []models.Topic{
					{ID: "go-type-params", ParentID: ptr("go-generics"), Slug: "go-type-params", Title: "Type Parameters", Description: "Generic functions and types, constraints", SortOrder: 1},
					{ID: "go-constraints", ParentID: ptr("go-generics"), Slug: "go-constraints", Title: "Constraints", Description: "comparable, any, custom constraints, union types", SortOrder: 2},
				},
			},
			{
				ID: "go-testing", ParentID: ptr("go"), Slug: "go-testing", Title: "Testing", Description: "Testing in Go", SortOrder: 7,
				Children: []models.Topic{
					{ID: "go-test-basics", ParentID: ptr("go-testing"), Slug: "go-test-basics", Title: "Test Basics", Description: "Test functions, table-driven tests, subtests", SortOrder: 1},
					{ID: "go-benchmarks", ParentID: ptr("go-testing"), Slug: "go-benchmarks", Title: "Benchmarks", Description: "Benchmark functions, profiling with pprof", SortOrder: 2},
				},
			},
		},
	},
	{
		ID: "javascript", Slug: "javascript", Title: "JavaScript", Description: "JavaScript programming", SortOrder: 2,
		Children: []models.Topic{
			{
				ID: "js-basics", ParentID: ptr("javascript"), Slug: "js-basics", Title: "Basics", Description: "JavaScript fundamentals", SortOrder: 1,
				Children: []models.Topic{
					{ID: "js-variables", ParentID: ptr("js-basics"), Slug: "js-variables", Title: "Variables & Types", Description: "let, const, var, primitive types, type coercion", SortOrder: 1},
					{ID: "js-functions", ParentID: ptr("js-basics"), Slug: "js-functions", Title: "Functions", Description: "Declarations, arrows, closures, this binding", SortOrder: 2},
					{ID: "js-objects", ParentID: ptr("js-basics"), Slug: "js-objects", Title: "Objects & Arrays", Description: "Object literals, destructuring, spread, array methods", SortOrder: 3},
					{ID: "js-control-flow", ParentID: ptr("js-basics"), Slug: "js-control-flow", Title: "Control Flow", Description: "if/else, loops, switch, optional chaining", SortOrder: 4},
				},
			},
			{
				ID: "js-async", ParentID: ptr("javascript"), Slug: "js-async", Title: "Async JavaScript", Description: "Asynchronous programming in JavaScript", SortOrder: 2,
				Children: []models.Topic{
					{ID: "js-promises", ParentID: ptr("js-async"), Slug: "js-promises", Title: "Promises", Description: "Promise creation, chaining, Promise.all, error handling", SortOrder: 1},
					{ID: "js-async-await", ParentID: ptr("js-async"), Slug: "js-async-await", Title: "Async/Await", Description: "async functions, await, error handling, parallel execution", SortOrder: 2},
					{ID: "js-event-loop", ParentID: ptr("js-async"), Slug: "js-event-loop", Title: "Event Loop", Description: "Call stack, task queue, microtasks, setTimeout", SortOrder: 3},
				},
			},
			{
				ID: "js-modules", ParentID: ptr("javascript"), Slug: "js-modules", Title: "Modules", Description: "JavaScript module systems", SortOrder: 3,
				Children: []models.Topic{
					{ID: "js-esm", ParentID: ptr("js-modules"), Slug: "js-esm", Title: "ES Modules", Description: "import/export, default exports, dynamic imports", SortOrder: 1},
				},
			},
		},
	},
	{
		ID: "typescript", Slug: "typescript", Title: "TypeScript", Description: "TypeScript type system", SortOrder: 3,
		Children: []models.Topic{
			{
				ID: "ts-basics", ParentID: ptr("typescript"), Slug: "ts-basics", Title: "Basics", Description: "TypeScript fundamentals", SortOrder: 1,
				Children: []models.Topic{
					{ID: "ts-types", ParentID: ptr("ts-basics"), Slug: "ts-types", Title: "Basic Types", Description: "Type annotations, inference, unions, intersections", SortOrder: 1},
					{ID: "ts-interfaces", ParentID: ptr("ts-basics"), Slug: "ts-interfaces", Title: "Interfaces & Types", Description: "Interface vs type, extending, implementing", SortOrder: 2},
					{ID: "ts-generics", ParentID: ptr("ts-basics"), Slug: "ts-generics", Title: "Generics", Description: "Generic functions, classes, constraints, utility types", SortOrder: 3},
				},
			},
			{
				ID: "ts-advanced", ParentID: ptr("typescript"), Slug: "ts-advanced", Title: "Advanced Types", Description: "Advanced TypeScript patterns", SortOrder: 2,
				Children: []models.Topic{
					{ID: "ts-mapped", ParentID: ptr("ts-advanced"), Slug: "ts-mapped", Title: "Mapped Types", Description: "Mapped types, conditional types, template literals", SortOrder: 1},
					{ID: "ts-narrowing", ParentID: ptr("ts-advanced"), Slug: "ts-narrowing", Title: "Type Narrowing", Description: "Type guards, discriminated unions, assertion functions", SortOrder: 2},
				},
			},
		},
	},
	{
		ID: "react", Slug: "react", Title: "React", Description: "React UI library", SortOrder: 4,
		Children: []models.Topic{
			{
				ID: "react-basics", ParentID: ptr("react"), Slug: "react-basics", Title: "Basics", Description: "React fundamentals", SortOrder: 1,
				Children: []models.Topic{
					{ID: "react-components", ParentID: ptr("react-basics"), Slug: "react-components", Title: "Components & JSX", Description: "Function components, JSX syntax, props, children", SortOrder: 1},
					{ID: "react-state", ParentID: ptr("react-basics"), Slug: "react-state", Title: "State & useState", Description: "useState hook, state updates, immutability", SortOrder: 2},
					{ID: "react-effects", ParentID: ptr("react-basics"), Slug: "react-effects", Title: "Effects & useEffect", Description: "useEffect hook, cleanup, dependency arrays", SortOrder: 3},
				},
			},
			{
				ID: "react-patterns", ParentID: ptr("react"), Slug: "react-patterns", Title: "Patterns", Description: "React design patterns", SortOrder: 2,
				Children: []models.Topic{
					{ID: "react-hooks", ParentID: ptr("react-patterns"), Slug: "react-hooks", Title: "Custom Hooks", Description: "Building custom hooks, hook composition, rules of hooks", SortOrder: 1},
					{ID: "react-context", ParentID: ptr("react-patterns"), Slug: "react-context", Title: "Context", Description: "createContext, useContext, provider patterns", SortOrder: 2},
				},
			},
		},
	},
}
