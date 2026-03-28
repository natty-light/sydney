use std::os::unix::io::RawFd;
use std::sync::Mutex;

static ORIGINAL_TERMIOS: Mutex<Option<libc::termios>> = Mutex::new(None);

#[no_mangle]
pub extern "C" fn sydney_term_enable_raw(fd: i64) -> i64 {
    let fd = fd as RawFd;
    unsafe {
        let mut original: libc::termios = std::mem::zeroed();
        if libc::tcgetattr(fd, &mut original) != 0 {
            return -1;
        }
        *ORIGINAL_TERMIOS.lock().unwrap() = Some(original);

        let mut raw = original;
        libc::cfmakeraw(&mut raw);
        if libc::tcsetattr(fd, libc::TCSANOW, &raw) != 0 {
            return -1;
        }
    }
    0
}

#[no_mangle]
pub extern "C" fn sydney_restore_state(fd: i64) -> i64 {
    let fd = fd as RawFd;
    let guard = ORIGINAL_TERMIOS.lock().unwrap();
    if let Some(ref original) = *guard {
        unsafe {
            if libc::tcsetattr(fd, libc::TCSANOW, original) != 0 {
                return -1;
            }
        }

        0
    } else {
        -1
    }
}
