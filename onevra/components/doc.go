// Package components provides product-neutral common controls built on MyGo.
// Callers own business state, localized text and actions. Native window and
// page composition belong to the separate shells/sidebar package.
//
// Element handles are MyGo values valid only during the current build pass.
// Do not retain them across frames; bind a ui.Handle for persistent identity.
//
// Common controls use MyGo's unstyled bases, which own pointer, keyboard, focus
// and accessibility semantics. Colors and sizes come from the frame theme (see
// the theme package); interaction states are read from the base every frame,
// and Draw and DrawOver provide Onevra's appearance. Disabled controls use the
// style's opacity treatment. The shells/sidebar package installs the theme before content
// builds; without Shell, callers can apply it through the theme package.
//
// Complex components live in independent subpackages: chart, command and
// chat. These may compose common controls; this package does not
// depend on its subpackages.
//
// The package covers everyday controls, not MyGo's full catalog. For anything
// else, compose MyGo's styled widgets or its bases directly. Import components
// alongside MyGo's ui package; neither requires an import alias.
package components
