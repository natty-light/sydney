use std::ffi::CStr;
use std::io::Write;
use std::os::raw::c_char;

fn flush() {
    let _ = std::io::stdout().flush();
}

#[no_mangle]
pub extern "C" fn sydney_print_int(val: i64) {
    print!("{}", val);
    flush();
}

#[no_mangle]
pub extern "C" fn sydney_print_float(val: f64) {
    print!("{}", val);
    flush();
}

#[no_mangle]
pub extern "C" fn sydney_print_bool(val: i8) {
    if val != 0 {
        print!("true");
    } else {
        print!("false");
    }
    flush();
}

#[no_mangle]
pub extern "C" fn sydney_print_string(ptr: *const c_char) {
    if ptr.is_null() {
        print!("null");
        return;
    }
    let s = unsafe { CStr::from_ptr(ptr) };
    print!("{}", s.to_str().unwrap_or("<invalid utf8>"));
    flush();
}

#[no_mangle]
pub extern "C" fn sydney_print_newline() {
    println!();
}

#[no_mangle]
pub extern "C" fn sydney_print_byte(val: u8) {
    print!("{}", val as char);
    flush();
}