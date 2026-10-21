#pragma once

#include <gtk/gtk.h>
#include <gtk-layer-shell.h>
#include <stdint.h>

gboolean boot_layer_supported(void);
void boot_layer_init(uintptr_t w);
void boot_focus(uintptr_t w);
void boot_round_corners(uintptr_t w, int width, int height, int radius);
void boot_raise(uintptr_t w);
void boot_x11_focus(uintptr_t w);
int boot_create_catcher(uintptr_t launcher);
int boot_wait_catcher_click(void);
