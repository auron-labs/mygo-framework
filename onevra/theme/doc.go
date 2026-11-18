// Package theme provides the Onevra design style's light and dark palettes:
// warm neutrals with a shared terracotta accent.
//
// The palettes are ui.Theme values applied through MyGo's own theme
// mechanism, so every MyGo widget keeps working unchanged. Applications
// choose light or dark with mygo.Theme.SetSource and choose the palette
// themselves: pass Onevra as a Shell's Theme option, install it with Apply,
// or supply a ui.Theme of their own.
package theme
