// sqlp is a powerputty package to provide extensions to sql.
//   - Consistent and minimal "single path" APIs.
//   - Contextual transactions to let you write tx agnostic methods cleanly.
//   - `reflect`ive scanning support using struct tags.
//   - Including nested struct and embedded struct support.
//   - `Repository` pattern support, to provide a wrapper around specific entities.
//   - Generic struct mapping scanning support to avoid sql tags for performance.
//   - TODO: Bare minimum, easy to understand query builders (glorified string builders, no extra DSL)
package sqlp
