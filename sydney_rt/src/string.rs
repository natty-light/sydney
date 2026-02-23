use std::ffi::CStr;
use std::os::raw::c_char;

#[no_mangle]
pub extern "C" fn sydney_strlen(ptr: *const c_char) -> i64 {
    if ptr.is_null() {
        return 0;
    }
    let s = unsafe { CStr::from_ptr(ptr) };
    s.to_bytes().len() as i64
}

#[no_mangle]
pub extern "C" fn sydney_strcat(a: *const c_char, b: *const c_char) -> *mut c_char {
    let sa = if a.is_null() {
        ""
    } else {
        unsafe { CStr::from_ptr(a) }.to_str().unwrap_or("")
    };
    let sb = if b.is_null() {
        ""
    } else {
        unsafe { CStr::from_ptr(b) }.to_str().unwrap_or("")
    };

    let mut result = String::with_capacity(sa.len() + sb.len() + 1);
    result.push_str(sa);
    result.push_str(sb);

    let c_string = std::ffi::CString::new(result).unwrap();
    c_string.into_raw() // caller (GC) is responsible for freeing
}