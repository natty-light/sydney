use std::collections::HashMap;
use std::ffi::CStr;
use std::path::Component::ParentDir;
use crate::gc::{sydney_gc_alloc, GC};

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

#[no_mangle]
pub extern "C" fn sydney_map_keys_int(map: *const HashMap<i64, i64>) -> *mut u8 {
  unsafe {
    let keys: Vec<i64> = (*map).keys().copied().collect();
    let len = keys.len() as i64;
    let buf = sydney_gc_alloc((keys.len() * 8) as i64);
    let data = buf as *mut i64;
    for (i, k) in keys.iter().enumerate() {
      *data.add(i) = *k;
    }
    // allocate header: { i64, ptr }
    let header = sydney_gc_alloc(16) as *mut i64;
    *header = len;
    *header.add(1) = buf as i64;
    header as *mut u8
  }
}

#[no_mangle]
pub extern "C" fn sydney_map_values_int(map: *const HashMap<i64, i64>) -> *mut u8 {
  unsafe {
    let vals: Vec<i64> = (*map).values().copied().collect();
    let len = vals.len() as i64;
    let buf = sydney_gc_alloc((vals.len() * 8) as i64);
    let data = buf as *mut i64;
    for (i, v) in vals.iter().enumerate() {
      *data.add(i) = *v;
    }
    let header = sydney_gc_alloc(16) as *mut i64;
    *header = len;
    *header.add(1) = buf as i64;
    header as *mut u8
  }
}

#[no_mangle]
pub extern "C" fn sydney_map_keys_str(map: *const HashMap<String, i64>) -> *mut u8 {
  unsafe {
    let keys: Vec<&String> = (*map).keys().collect();
    let len = keys.len() as i64;
    let buf = sydney_gc_alloc((keys.len() * 8) as i64);
    let data = buf as *mut i64;
    for (i, k) in keys.iter().enumerate() {
      let cstr = std::ffi::CString::new(k.as_str()).unwrap();
      let ptr = sydney_gc_alloc((k.len() + 1) as i64);
      std::ptr::copy_nonoverlapping(cstr.as_ptr(), ptr as *mut i8, k.len() + 1);
      *data.add(i) = ptr as i64;
    }
    let header = sydney_gc_alloc(16) as *mut i64;
    *header = len;
    *header.add(1) = buf as i64;
    header as *mut u8
  }
}

#[no_mangle]
pub extern "C" fn sydney_map_values_str(map: *const HashMap<String, i64>) -> *mut u8 {
  unsafe {
    let vals: Vec<i64> = (*map).values().copied().collect();
    let len = vals.len() as i64;
    let buf = sydney_gc_alloc((vals.len() * 8) as i64);
    let data = buf as *mut i64;
    for (i, v) in vals.iter().enumerate() {
      *data.add(i) = *v;
    }
    let header = sydney_gc_alloc(16) as *mut i64;
    *header = len;
    *header.add(1) = buf as i64;
    header as *mut u8
  }
}

#[no_mangle]
pub extern "C" fn sydney_map_contains_int(map: *const HashMap<i64, i64>, key: i64) -> i8 {
  unsafe { if (*map).contains_key(&key) { 1 } else { 0 } }
}

#[no_mangle]
pub extern "C" fn sydney_map_contains_str(map: *const HashMap<String, i64>, key: *const u8) -> i8 {
  unsafe {
    let k = CStr::from_ptr(key as *const i8).to_string_lossy();
    if (*map).contains_key(k.as_ref()) { 1 } else { 0 }
  }
}
