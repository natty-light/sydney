use std::fs;
use std::ffi::{CStr, CString};
use std::fs::File;
use std::os::raw::c_char;

#[cfg(unix)]
use std::os::unix::io::FromRawFd;

#[no_mangle]
pub extern "C" fn sydney_file_open(path: *const c_char) -> i64 {
    if path.is_null() {
        return -1;
    }

    let path: &str = unsafe { CStr::from_ptr(path) }.to_str().unwrap_or("");
    use std::fs::OpenOptions;
    match OpenOptions::new()
        .read(true)
        .write(true)
        .open(path) {
        Ok(file) => {
            use std::os::unix::io::IntoRawFd;
            file.into_raw_fd() as i64
        }
        Err(_) => -1,
    }
}

#[no_mangle]
pub extern "C" fn sydney_file_read(fd: i64) -> *const c_char {
    use std::io::Read;
    let mut file = unsafe { File::from_raw_fd(fd as i32) };
    let mut contents = String::new();
    match file.read_to_string(&mut contents) {
        Ok(_) => {
            std::mem::forget(file);
            match CString::new(contents.as_str()) {
                Ok(cs) => cs.into_raw(),
                Err(_) => std::ptr::null(),
            }
        }
        Err(_) => {
            std::mem::forget(file);
            std::ptr::null()
        },
    }
}

#[no_mangle]
pub extern "C" fn sydney_file_write(fd: i64, data: *const c_char) -> i64 {
    use std::io::Write;
    if data.is_null() {
        return -1;
    }
    let s = unsafe { CStr::from_ptr(data) }.to_bytes();
    let mut file = unsafe { std::fs::File::from_raw_fd(fd as i32) };
    let result = match file.write_all(s) {
        Ok(_) => 0,
        Err(_) => -1,
    };
    std::mem::forget(file);
    result
}

#[no_mangle]
pub extern "C" fn sydney_file_close(fd: i64) -> i64 {
    // Reconstructing the File and dropping it closes the fd
    let file = unsafe { std::fs::File::from_raw_fd(fd as i32) };
    drop(file);
    0
}
