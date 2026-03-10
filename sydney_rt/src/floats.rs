use std::f64::NAN;
use std::ffi::{CStr, CString};
use crate::error::LAST_ERROR;

#[no_mangle]
pub extern "C" fn sydney_atof(s: *const u8) -> f64 {
    unsafe {
        let cstr = CStr::from_ptr(s as *const i8);
        let result = cstr.to_str().unwrap_or("0").parse::<f64>().unwrap_or(f64::NAN);
        if result.is_nan() {
            LAST_ERROR.with(|e| {
                *e.borrow_mut() = Some(CString::new("invalid string").unwrap());
            });
            return f64::NAN
        }
        result
    }
}
