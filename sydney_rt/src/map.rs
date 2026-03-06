use std::collections::HashMap;
use std::ffi::CStr;
use std::path::Component::ParentDir;
use crate::gc::GC;

#[no_mangle]
pub extern "C" fn sydney_map_create_int() -> *mut HashMap<i64, i64> {
  let map = Box::into_raw(Box::new(HashMap::new()));
  unsafe {
    let gc = GC.as_mut().unwrap();
    gc.maps.push(map)
  }
  map
}
#[no_mangle]
pub extern "C" fn sydney_map_set_int(map: *mut HashMap<i64, i64>, key: i64, value: i64) {
  unsafe { (*map).insert(key, value); }
}
#[no_mangle]
pub extern "C" fn sydney_map_get_int(map: *const HashMap<i64, i64>, key: i64) -> i64 {
  unsafe { *(*map).get(&key).unwrap_or(&0) }
}
#[no_mangle]
pub extern "C" fn sydney_map_create_string() -> *mut HashMap<String, i64> {
  let map = Box::into_raw(Box::new(HashMap::new()));
  unsafe {
    let gc = GC.as_mut().unwrap();
    gc.maps.push(map)
  }
  map
}
#[no_mangle]
pub extern "C" fn sydney_map_set_str(map: *mut HashMap<String, i64>, key: *const u8, value: i64) {
  unsafe {
    let k = CStr::from_ptr(key as *const i8).to_string_lossy().into_owned();
    (*map).insert(k, value);
  }
}

#[no_mangle]
pub extern "C" fn sydney_map_get_str(map: *const HashMap<String, i64>, key: *const u8) -> i64 {
  unsafe {
    let k = CStr::from_ptr(key as *const i8).to_string_lossy();
    *(*map).get(k.as_ref()).unwrap_or(&0)
  }
}

#[no_mangle]
pub extern "C" fn sydney_map_get_destroy_int(map: *mut HashMap<i64, i64>) {
  unsafe {
    let gc = GC.as_mut().unwrap();
    gc.maps.retain(|m| (*m as *const () as usize) != (map as *const () as usize));
    drop(Box::from_raw(map));
  }
}

#[no_mangle]
pub extern "C" fn sydney_map_get_destroy_string(map: *mut HashMap<String, i64>) {
  unsafe {
    let gc = GC.as_mut().unwrap();
    gc.maps.retain(|m| (*m as *const () as usize != (map as *const () as usize)));
    drop(Box::from_raw(map));
  }
}