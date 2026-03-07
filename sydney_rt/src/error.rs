use std::cell::RefCell;
use std::ffi::CString;
use std::os::raw::c_char;

thread_local! {
      pub static LAST_ERROR: RefCell<Option<CString>> = RefCell::new(None);
  }

#[no_mangle]
pub extern "C" fn sydney_get_last_error() -> *const c_char {
    LAST_ERROR.with(|e| {
        match e.borrow().as_ref() {
            Some(s) => s.as_ptr(),
            None => std::ptr::null(),
        }
    })
}
