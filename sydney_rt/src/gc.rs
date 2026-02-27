use std::alloc::{alloc, dealloc, Layout};
use std::cmp::{max, min};
use std::collections::HashMap;

struct GcState {
    allocations: HashMap<usize, Allocation>,  // ptr address → index in allocations
    global_roots: Vec<*const *mut u8>,
    bytes_allocated: usize,
    threshold: usize,
}

impl GcState {
    fn new() -> Self {
        GcState {
            global_roots: Vec::new(),
            bytes_allocated: 0,
            threshold: 1024 * 1024,
            allocations: HashMap::new(),
        }
    }
}

struct Allocation {
    ptr: *mut u8,
    size: usize,
    marked: bool,
}


static mut GC: Option<GcState> = None;

#[no_mangle]
pub extern "C" fn sydney_gc_init() {
  unsafe {
      GC = Some(GcState::new())
  }
}

#[no_mangle]
pub extern "C" fn sydney_gc_alloc(size: i64) -> *mut u8 {
    unsafe {
       if GC.as_ref().unwrap().bytes_allocated >= GC.as_ref().unwrap().threshold {
           sydney_gc_collect();
       }

        if size <= 0 {
            return std::ptr::null_mut();
        }

        let gc = GC.as_mut().unwrap();

        let layout = Layout::from_size_align(size as usize, 8).unwrap();
        let ptr = alloc(layout);

        if ptr.is_null() {
            panic!("sydney_gc_alloc: out of memory");
        }

        let as_usize = size as usize;
        // scary cast but on this system i64 is the size of ptr so should be usize
        gc.allocations.insert(ptr as usize,Allocation{ ptr, size: as_usize, marked: false });
        gc.bytes_allocated += as_usize;

        ptr
    }
}

#[no_mangle]
pub extern "C" fn sydney_gc_collect() {
    unsafe {
        let gc = GC.as_mut().unwrap();

        let anchor: usize = 0;
        let stack_top = &anchor as *const usize as *mut u8;
        // this will only work on macOS
        let stack_base = libc::pthread_get_stackaddr_np(libc::pthread_self()) as *mut u8;

        let low = min(stack_top as usize, stack_base as usize);
        let high = max(stack_top as usize, stack_base as usize);


        let mut stack_roots: Vec<*mut u8> = Vec::new();
        for addr in (low..high).step_by(8) {
            let value = *(addr as *const usize);
            if gc.allocations.contains_key(&value) {
                stack_roots.push(value as *mut u8);
            }
        }

        // mark
        for root_addr in gc.global_roots.iter() {
            let heap_ptr: *mut u8 = **root_addr;
            mark(heap_ptr);
        }

        for ptr in stack_roots.iter() {
            mark(*ptr);
        }



        let before = gc.allocations.len();
        // sweep
        gc.allocations.retain(|_, alloc| {
            if alloc.marked {
                alloc.marked = false;
                true
            } else {
                let layout = Layout::from_size_align(alloc.size as usize, 8).unwrap();
                dealloc(alloc.ptr, layout);
                gc.bytes_allocated -= alloc.size;
                false
            }
        });
        let after = gc.allocations.len();
        // eprintln!("GC: {} total, {} swept, {} kept", before, before-after, after);

    }
}

#[no_mangle]
pub extern "C" fn sydney_gc_add_global_root(_root: *const *mut u8) {
    unsafe {
        let gc = GC.as_mut().unwrap();
        gc.global_roots.push(_root);
    }
}

#[no_mangle]
pub extern "C" fn sydney_gc_shutdown() {
    unsafe {
        let gc = GC.as_mut().unwrap();
        for (_, alloc) in gc.allocations.drain() {
            let layout = Layout::from_size_align(alloc.size as usize, 8).unwrap();
            dealloc(alloc.ptr, layout);
        }
        gc.global_roots.clear();
        gc.bytes_allocated = 0;
    }

}

fn mark(heap_ptr: *mut u8) {
    unsafe {
        let gc = GC.as_mut().unwrap();
        if let Some(a) = gc.allocations.get_mut(&(heap_ptr as usize))  {
            if a.marked {
                return
            }
            a.marked = true;

            scan_range(a.ptr, a.ptr.add(a.size));
        }
    }
}

fn scan_range(start: *mut u8, end: *mut u8) {
   unsafe {
       let low = min(start as usize, end as usize);
       let high = max(start as usize, end as usize);

       for addr in (low..high).step_by(8) {
           let value = *(addr as *const usize);
           mark(value as *mut u8);
       }
   }
}