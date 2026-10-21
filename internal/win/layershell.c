#include "layershell.h"
#include <math.h>
#include <gdk/gdkx.h>
#include <X11/Xlib.h>

gboolean boot_layer_supported(void) {
	return gtk_layer_is_supported();
}

void boot_layer_init(uintptr_t raw) {
	GtkWindow *w = (GtkWindow *)raw;
	gtk_layer_init_for_window(w);
	gtk_layer_set_namespace(w, "boot-menu");
	gtk_layer_set_layer(w, GTK_LAYER_SHELL_LAYER_OVERLAY);
	gtk_layer_set_keyboard_mode(w, GTK_LAYER_SHELL_KEYBOARD_MODE_EXCLUSIVE);
	gtk_layer_set_exclusive_zone(w, -1);
}

void boot_focus(uintptr_t raw) {
	GdkWindow *w = (GdkWindow *)raw;
	gdk_window_focus(w, GDK_CURRENT_TIME);
}

void boot_round_corners(uintptr_t raw, int width, int height, int radius) {
	GdkWindow *w = (GdkWindow *)raw;
	cairo_surface_t *mask = cairo_image_surface_create(CAIRO_FORMAT_A1, width, height);
	cairo_t *cr = cairo_create(mask);
	cairo_set_operator(cr, CAIRO_OPERATOR_SOURCE);
	cairo_set_source_rgba(cr, 0, 0, 0, 0);
	cairo_paint(cr);
	cairo_set_operator(cr, CAIRO_OPERATOR_OVER);
	cairo_set_source_rgba(cr, 1, 1, 1, 1);
	double r = radius, x = 0, y = 0, ww = width, hh = height;
	const double hp = 1.5707963267948966;
	cairo_new_path(cr);
	cairo_arc(cr, x + ww - r, y + r, r, -hp, 0);
	cairo_arc(cr, x + ww - r, y + hh - r, r, 0, hp);
	cairo_arc(cr, x + r, y + hh - r, r, hp, 2 * hp);
	cairo_arc(cr, x + r, y + r, r, 2 * hp, 3 * hp);
	cairo_close_path(cr);
	cairo_fill(cr);
	cairo_region_t *region = gdk_cairo_region_create_from_surface(mask);
	gdk_window_shape_combine_region(w, region, 0, 0);
	cairo_region_destroy(region);
	cairo_destroy(cr);
	cairo_surface_destroy(mask);
}


static Display *catcher_dpy = NULL;
static Window catcher_win = 0;

int boot_create_catcher(uintptr_t launcher) {
	catcher_dpy = XOpenDisplay(NULL);
	if (!catcher_dpy) {
		return 0;
	}
	int screen = DefaultScreen(catcher_dpy);
	Window root = RootWindow(catcher_dpy, screen);
	XSetWindowAttributes attrs;
	attrs.override_redirect = True;
	attrs.event_mask = ButtonPressMask;
	catcher_win = XCreateWindow(catcher_dpy, root, 0, 0,
		DisplayWidth(catcher_dpy, screen), DisplayHeight(catcher_dpy, screen),
		0, 0, InputOnly, CopyFromParent,
		CWOverrideRedirect | CWEventMask, &attrs);
	XMapRaised(catcher_dpy, catcher_win);
	XRaiseWindow(catcher_dpy, (Window)launcher);
	XFlush(catcher_dpy);
	return 1;
}

int boot_wait_catcher_click(void) {
	if (!catcher_dpy) {
		return 0;
	}
	for (;;) {
		XEvent ev;
		XNextEvent(catcher_dpy, &ev);
		if (ev.type == ButtonPress) {
			XDestroyWindow(catcher_dpy, catcher_win);
			XCloseDisplay(catcher_dpy);
			catcher_dpy = NULL;
			catcher_win = 0;
			return 1;
		}
	}
}

void boot_raise(uintptr_t raw) {
	gdk_window_raise((GdkWindow *)raw);
}

void boot_x11_focus(uintptr_t raw) {
	GdkWindow *w = (GdkWindow *)raw;
	Display *dpy = GDK_WINDOW_XDISPLAY(w);
	Window xid = GDK_WINDOW_XID(w);
	XRaiseWindow(dpy, xid);
	XSetInputFocus(dpy, xid, RevertToParent, CurrentTime);
	XFlush(dpy);
}

