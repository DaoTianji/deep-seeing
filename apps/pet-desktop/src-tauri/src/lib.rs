use std::sync::Mutex;

use tauri::{AppHandle, Manager, PhysicalPosition, PhysicalSize, Runtime, State};
use tauri_plugin_opener::OpenerExt;

const ORB_SIZE: u32 = 72;
const PANEL_WIDTH: u32 = 520;
const PANEL_HEIGHT: u32 = 680;
const PANEL_MIN_WIDTH: u32 = 380;
const PANEL_MIN_HEIGHT: u32 = 460;
const MARGIN: i32 = 18;

struct PetWindowState {
    panel_size: Mutex<PhysicalSize<u32>>,
}

impl Default for PetWindowState {
    fn default() -> Self {
        Self {
            panel_size: Mutex::new(PhysicalSize::new(PANEL_WIDTH, PANEL_HEIGHT)),
        }
    }
}

fn resize_around_current_anchor<R: Runtime>(
    window: &tauri::WebviewWindow<R>,
    width: u32,
    height: u32,
) -> Result<(), String> {
    let current_size = window.outer_size().map_err(|e| e.to_string())?;
    let current_position = window.outer_position().map_err(|e| e.to_string())?;
    let anchor_x = current_position.x + current_size.width as i32;
    let anchor_y = current_position.y + current_size.height as i32;

    let monitor = window
        .current_monitor()
        .map_err(|e| e.to_string())?
        .or(window.primary_monitor().map_err(|e| e.to_string())?)
        .ok_or_else(|| "no monitor found".to_string())?;
    let screen = monitor.size();
    let screen_position = monitor.position();
    let max_x = screen_position.x + screen.width as i32 - width as i32;
    let max_y = screen_position.y + screen.height as i32 - height as i32;
    let x = (anchor_x - width as i32).clamp(screen_position.x, max_x);
    let y = (anchor_y - height as i32).clamp(screen_position.y, max_y);

    window
        .set_size(PhysicalSize::new(width, height))
        .map_err(|e| e.to_string())?;
    window
        .set_position(PhysicalPosition::new(x, y))
        .map_err(|e| e.to_string())?;
    Ok(())
}

fn main_window<R: Runtime>(app: &AppHandle<R>) -> Result<tauri::WebviewWindow<R>, String> {
    app.get_webview_window("main")
        .ok_or_else(|| "main window missing".to_string())
}

#[tauri::command]
fn set_pet_mode_orb(app: AppHandle, state: State<'_, PetWindowState>) -> Result<(), String> {
    let window = main_window(&app)?;
    let current = window.outer_size().map_err(|e| e.to_string())?;
    if current.width >= PANEL_MIN_WIDTH && current.height >= PANEL_MIN_HEIGHT {
        *state.panel_size.lock().map_err(|e| e.to_string())? = current;
    }
    window
        .set_min_size::<PhysicalSize<u32>>(None)
        .map_err(|e| e.to_string())?;
    resize_around_current_anchor(&window, ORB_SIZE, ORB_SIZE)?;
    window.set_resizable(false).map_err(|e| e.to_string())
}

#[tauri::command]
fn set_pet_mode_panel(app: AppHandle, state: State<'_, PetWindowState>) -> Result<(), String> {
    let window = main_window(&app)?;
    let size = *state.panel_size.lock().map_err(|e| e.to_string())?;
    window.set_resizable(true).map_err(|e| e.to_string())?;
    window
        .set_min_size(Some(PhysicalSize::new(PANEL_MIN_WIDTH, PANEL_MIN_HEIGHT)))
        .map_err(|e| e.to_string())?;
    resize_around_current_anchor(&window, size.width, size.height)
}

#[tauri::command]
fn open_full_room(app: AppHandle) -> Result<(), String> {
    app.opener()
        .open_url("http://127.0.0.1:8787/", None::<&str>)
        .map_err(|e| e.to_string())
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .manage(PetWindowState::default())
        .plugin(tauri_plugin_opener::init())
        .invoke_handler(tauri::generate_handler![
            set_pet_mode_orb,
            set_pet_mode_panel,
            open_full_room
        ])
        .setup(|app| {
            if let Ok(window) = main_window(app.handle()) {
                let monitor = window
                    .primary_monitor()?
                    .ok_or_else(|| std::io::Error::other("no primary monitor"))?;
                let screen = monitor.size();
                let pos = monitor.position();
                let x = pos.x + screen.width as i32 - ORB_SIZE as i32 - MARGIN;
                let y = pos.y + screen.height as i32 - ORB_SIZE as i32 - MARGIN;
                window.set_position(PhysicalPosition::new(x, y))?;
                window.set_resizable(false)?;
            }
            Ok(())
        })
        .run(tauri::generate_context!())
        .expect("error while running pet-desktop");
}
