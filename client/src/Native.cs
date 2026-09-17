using System;
using System.Runtime.InteropServices;

namespace Foghorn
{
    /// <summary>
    /// The handful of Windows API calls WinForms does not expose. Every call is
    /// wrapped so that a failure (or running under Mono for testing) is harmless.
    /// </summary>
    static class Native
    {
        public const int WS_EX_TOPMOST = 0x00000008;
        public const int WS_EX_TOOLWINDOW = 0x00000080;
        public const int WS_EX_NOACTIVATE = 0x08000000;
        public const int CS_DROPSHADOW = 0x00020000;
        public const int WM_MOUSEACTIVATE = 0x0021;
        public const int MA_NOACTIVATE = 3;

        static readonly IntPtr HWND_TOPMOST = new IntPtr(-1);
        const uint SWP_NOSIZE = 0x0001, SWP_NOMOVE = 0x0002, SWP_NOACTIVATE = 0x0010, SWP_NOOWNERZORDER = 0x0200;

        [DllImport("user32.dll")]
        static extern bool SetProcessDPIAware();

        [DllImport("user32.dll")]
        static extern bool SetWindowPos(IntPtr hWnd, IntPtr after, int x, int y, int cx, int cy, uint flags);

        [DllImport("user32.dll")]
        static extern bool SetForegroundWindow(IntPtr hWnd);

        [DllImport("dwmapi.dll")]
        static extern int DwmSetWindowAttribute(IntPtr hwnd, int attr, ref int value, int size);

        /// <summary>Without this, Windows bitmap-stretches the alert on high-DPI screens and the text goes blurry.</summary>
        public static void MakeDpiAware()
        {
            try { SetProcessDPIAware(); } catch { }
        }

        /// <summary>Push the window back above everything else without taking focus.</summary>
        public static void KeepOnTop(IntPtr hwnd)
        {
            try { SetWindowPos(hwnd, HWND_TOPMOST, 0, 0, 0, 0, SWP_NOMOVE | SWP_NOSIZE | SWP_NOACTIVATE | SWP_NOOWNERZORDER); } catch { }
        }

        public static void BringToFront(IntPtr hwnd)
        {
            try { SetForegroundWindow(hwnd); } catch { }
        }

        /// <summary>Windows 11 rounded corners. Older Windows returns an error code, which we ignore.</summary>
        public static void RoundCorners(IntPtr hwnd)
        {
            try { int round = 2; DwmSetWindowAttribute(hwnd, 33, ref round, sizeof(int)); } catch { }
        }
    }
}
