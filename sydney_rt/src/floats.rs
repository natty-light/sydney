use crate::error::LAST_ERROR;
use std::ffi::{CStr, CString};
use std::os::raw::c_char;

#[no_mangle]
pub extern "C" fn sydney_atof(s: *const u8) -> f64 {
    unsafe {
        let cstr = CStr::from_ptr(s as *const i8);
        let result = cstr
            .to_str()
            .unwrap_or("0")
            .parse::<f64>()
            .unwrap_or(f64::NAN);
        if result.is_nan() {
            LAST_ERROR.with(|e| {
                *e.borrow_mut() = Some(CString::new("invalid string").unwrap());
            });
            return f64::NAN;
        }
        result
    }
}

#[no_mangle]
pub extern "C" fn sydney_ftoa(f: f64) -> *mut c_char {
    let result = f.to_string();
    let c_string = std::ffi::CString::new(result).unwrap();
    c_string.into_raw()
}
