// Stub for Astro dev server — replaced by Wails' real runtime at runtime.
// Used only when opening http://localhost:4321 directly in a browser.

export const Events = {
  On: (name, _cb) => console.log('[wails-stub] Events.On:', name),
  Emit: (name, data) => console.log('[wails-stub] Events.Emit:', name, data),
  Off: (name) => console.log('[wails-stub] Events.Off:', name),
};

export const Call = {
  ByName: async (name, ...args) => {
    console.log('[wails-stub] Call.ByName:', name, args);
    switch (name) {
      case 'main.AppService.GetMonitors':
        return [
          { index: 0, width: 1920, height: 1080, x: 0,    y: 0, primary: true  },
          { index: 1, width: 1080, height: 1920, x: -1080, y: -420, primary: false },
        ];
      case 'main.AppService.GetVersion':
        return '0.0.0-dev';
      case 'main.AppService.CheckDependencies':
        return { ffmpeg: true, ffprobe: true, mpv: true };
      case 'main.AppService.FileExists':
        return false;
      case 'main.AppService.BrowseFile':
        return '';
      case 'main.AppService.IsVideoFile':
        return false;
      case 'main.AppService.GetThumbnail':
        return '';
      default:
        return null;
    }
  },
};
