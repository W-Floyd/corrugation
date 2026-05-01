import { defineStore } from "pinia";
import { ref, shallowRef, watch } from "vue";

export const useCameraStore = defineStore("camera", () => {
  const opened = ref(false);
  const stream = shallowRef<MediaStream | null>(null);
  const callback = shallowRef<((files: File[]) => void) | null>(null);
  const previewUrl = ref<string | null>(null);
  const pendingFile = shallowRef<File | null>(null);
  const _originalBlob = shallowRef<Blob | null>(null);
  const devices = ref<MediaDeviceInfo[]>([]);
  const selectedDeviceId = ref<string | null>(null);
  let rotation = 0;

  // Load saved camera device from localStorage
  selectedDeviceId.value = localStorage.getItem("camera_device_id");
  watch(
    () => selectedDeviceId.value,
    (newId) => {
      localStorage.setItem("camera_device_id", newId ?? "");
    },
  );
  let landscape = false;
  let _beta: number | null = null;
  let _gamma: number | null = null;

  const _onResize = (): void => {
    if (_beta !== null) return;
    const mobile = navigator.maxTouchPoints > 0;
    landscape = mobile && window.innerWidth > window.innerHeight;
  };

  const _onOrientation = (e: DeviceOrientationEvent): void => {
    _beta = e.beta;
    _gamma = e.gamma;
    const gAbs = Math.abs(e.gamma ?? 0);
    const bAbs = Math.abs(e.beta ?? 0);
    if (landscape) {
      if (bAbs > gAbs + 15) landscape = false;
    } else {
      if (gAbs > bAbs + 15) landscape = true;
    }
  };

  const _startOrientation = async (): Promise<void> => {
    // DeviceOrientationEvent.requestPermission is an iOS Safari extension not in standard TS types
    const DeviceOrientationEventIOS =
      DeviceOrientationEvent as typeof DeviceOrientationEvent & {
        requestPermission?: () => Promise<string>;
      };
    if (
      typeof DeviceOrientationEventIOS !== "undefined" &&
      typeof DeviceOrientationEventIOS.requestPermission === "function"
    ) {
      try {
        const perm = await DeviceOrientationEventIOS.requestPermission();
        if (perm === "granted") {
          window.addEventListener("deviceorientation", _onOrientation);
        }
      } catch (e) {
        console.warn("Device orientation permission denied:", e);
      }
    } else if (typeof DeviceOrientationEvent !== "undefined") {
      window.addEventListener("deviceorientation", _onOrientation);
    }
  };

  async function loadDevices(): Promise<void> {
    try {
      const allDevices = await navigator.mediaDevices.enumerateDevices();
      devices.value = allDevices.filter((d) => d.kind === "videoinput");
    } catch (e) {
      console.warn("Failed to enumerate devices:", e);
    }
  }

  async function open(
    cb: (files: File[]) => void,
    deviceId?: string,
  ): Promise<void> {
    landscape = window.innerWidth > window.innerHeight;
    _beta = null;
    _gamma = null;
    window.addEventListener("resize", _onResize);
    window.addEventListener("orientationchange", _onResize);
    await _startOrientation();
    callback.value = cb;
    previewUrl.value = null;
    pendingFile.value = null;
    _originalBlob.value = null;
    rotation = 0;
    try {
      // Check if camera device exists before attempting to open
      const devices = await navigator.mediaDevices.enumerateDevices();
      const hasCamera = devices.some((device) => device.kind === "videoinput");
      if (!hasCamera) {
        console.warn("No camera device found");
        _reset();
        return;
      }

      const mobile = navigator.maxTouchPoints > 0;
      const portrait = mobile && window.innerHeight > window.innerWidth;
      const constraints: MediaTrackConstraints = {};
      // Use saved device ID if no deviceId provided
      const actualDeviceId = deviceId ?? selectedDeviceId.value;
      if (actualDeviceId) {
        constraints.deviceId = { ideal: actualDeviceId };
      } else {
        constraints.facingMode = mobile ? "environment" : "user";
      }
      constraints.width = { ideal: portrait ? 2160 : 3840 };
      constraints.height = { ideal: portrait ? 3840 : 2160 };
      constraints.aspectRatio = { ideal: 16 / 9 };
      stream.value = await navigator.mediaDevices.getUserMedia({
        video: constraints,
        audio: false,
      });
      opened.value = true;
    } catch (e) {
      console.error("Camera error:", e);
      _reset();
    }
  }

  function _devicePortrait(): boolean {
    if (_beta !== null && _gamma !== null) {
      return Math.abs(_beta) > Math.abs(_gamma);
    }
    return (
      navigator.maxTouchPoints > 0 && window.innerHeight > window.innerWidth
    );
  }

  function _rotateAngle(): number {
    if (_gamma !== null) {
      return _gamma < 0 ? -Math.PI / 2 : Math.PI / 2;
    }
    const angle = (screen.orientation?.angle ?? 0) as number;
    return angle === 270 ? Math.PI / 2 : -Math.PI / 2;
  }

  function capture(): void {
    const video = document.getElementById("cameraVideo") as HTMLVideoElement;
    const canvas = document.getElementById("cameraCanvas") as HTMLCanvasElement;
    if (!video || !canvas) return;

    const vw = video.videoWidth;
    const vh = video.videoHeight;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    const streamPortrait = vh > vw;
    const devicePortrait = _devicePortrait();

    if (streamPortrait !== devicePortrait) {
      const angle = _rotateAngle();
      canvas.width = vh;
      canvas.height = vw;
      ctx.save();
      ctx.translate(canvas.width / 2, canvas.height / 2);
      ctx.rotate(angle);
      ctx.drawImage(video, -vw / 2, -vh / 2);
      ctx.restore();
    } else {
      canvas.width = vw;
      canvas.height = vh;
      ctx.drawImage(video, 0, 0);
    }

    if (stream.value) {
      stream.value.getTracks().forEach((t) => t.stop());
      stream.value = null;
    }
    rotation = 0;
    canvas.toBlob(
      (blob) => {
        if (!blob) return;
        _originalBlob.value = blob;
        pendingFile.value = new File([blob], "photo.jpg", {
          type: "image/jpeg",
        });
        previewUrl.value = URL.createObjectURL(blob);
      },
      "image/jpeg",
      0.92,
    );
  }

  function rotate(): void {
    rotation = (rotation + 90) % 360;
    if (!_originalBlob.value) return;

    const blob = _originalBlob.value;
    const img = new Image();
    img.onload = () => {
      const canvas = document.getElementById(
        "cameraCanvas",
      ) as HTMLCanvasElement;
      if (!canvas) return;

      const swap = rotation % 180 !== 0;
      canvas.width = swap ? img.height : img.width;
      canvas.height = swap ? img.width : img.height;

      const ctx = canvas.getContext("2d");
      if (!ctx) return;

      ctx.save();
      ctx.translate(canvas.width / 2, canvas.height / 2);
      ctx.rotate((rotation * Math.PI) / 180);
      ctx.drawImage(img, -img.width / 2, -img.height / 2);
      ctx.restore();

      URL.revokeObjectURL(img.src);
      if (previewUrl.value) URL.revokeObjectURL(previewUrl.value);

      canvas.toBlob(
        (rotatedBlob) => {
          if (!rotatedBlob) return;
          pendingFile.value = new File([rotatedBlob], "photo.jpg", {
            type: "image/jpeg",
          });
          previewUrl.value = URL.createObjectURL(rotatedBlob);
        },
        "image/jpeg",
        0.92,
      );
    };
    img.src = URL.createObjectURL(blob);
  }

  function confirm(): void {
    if (callback.value && pendingFile.value) {
      callback.value([pendingFile.value]);
    }
    _reset();
  }

  async function retake(): Promise<void> {
    if (previewUrl.value) URL.revokeObjectURL(previewUrl.value);
    previewUrl.value = null;
    pendingFile.value = null;
    _originalBlob.value = null;
    rotation = 0;
    try {
      const mobile = navigator.maxTouchPoints > 0;
      const portrait = mobile && window.innerHeight > window.innerWidth;
      const videoConstraints: MediaTrackConstraints = {};
      if (mobile) {
        videoConstraints.facingMode = "environment";
      }
      videoConstraints.width = { ideal: portrait ? 2160 : 3840 };
      videoConstraints.height = { ideal: portrait ? 3840 : 2160 };
      videoConstraints.aspectRatio = { ideal: 16 / 9 };
      stream.value = await navigator.mediaDevices.getUserMedia({
        video: videoConstraints,
        audio: false,
      });
    } catch (e) {
      console.error("Camera error:", e);
    }
  }

  function close(): void {
    _reset();
  }

  function _reset(): void {
    window.removeEventListener("resize", _onResize);
    window.removeEventListener("orientationchange", _onResize);
    window.removeEventListener("deviceorientation", _onOrientation);
    _beta = null;
    _gamma = null;
    if (stream.value) {
      stream.value.getTracks().forEach((t) => t.stop());
      stream.value = null;
    }
    if (previewUrl.value) URL.revokeObjectURL(previewUrl.value);
    previewUrl.value = null;
    pendingFile.value = null;
    _originalBlob.value = null;
    rotation = 0;
    // Keep selectedDeviceId to remember last camera
    opened.value = false;
  }

  return {
    opened,
    stream,
    callback,
    previewUrl,
    pendingFile,
    devices,
    selectedDeviceId,
    loadDevices,
    rotation,
    open,
    capture,
    rotate,
    confirm,
    retake,
    close,
  };
});
