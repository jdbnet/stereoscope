package com.wails.app;

import android.app.Activity;
import android.content.Context;
import android.content.Intent;
import android.media.AudioAttributes;
import android.media.AudioDeviceInfo;
import android.media.AudioManager;
import android.media.AudioPlaybackConfiguration;
import android.os.Build;

import org.json.JSONArray;
import org.json.JSONObject;

import java.util.List;

/**
 * JNI entry point for the Go audio backend.
 * Selecting a device on Android monitors the active route, not a private mix.
 */
public final class AudioBridge {
    static final int REQUEST_CAPTURE = 7401;

    private static MainActivity activity;

    static {
        System.loadLibrary("wails");
    }

    private AudioBridge() {}

    public static void attach(MainActivity host) {
        activity = host;
        nativeRegister();
        nativeSetCaptureState("idle");
    }

    public static void requestCapture() {
        if (activity == null) {
            nativeSetCaptureState("denied");
            return;
        }
        if (Build.VERSION.SDK_INT < 29) {
            nativeSetCaptureState("unsupported");
            return;
        }
        activity.runOnUiThread(() -> {
            android.media.projection.MediaProjectionManager manager =
                    (android.media.projection.MediaProjectionManager)
                            activity.getSystemService(Context.MEDIA_PROJECTION_SERVICE);
            if (manager == null) {
                nativeSetCaptureState("denied");
                return;
            }
            nativeSetCaptureState("need-consent");
            activity.startActivityForResult(manager.createScreenCaptureIntent(), REQUEST_CAPTURE);
        });
    }

    public static boolean handleActivityResult(int requestCode, int resultCode, Intent data) {
        if (requestCode != REQUEST_CAPTURE) {
            return false;
        }
        if (resultCode != Activity.RESULT_OK || data == null || activity == null) {
            nativeSetCaptureState("denied");
            return true;
        }
        Intent service = new Intent(activity, AudioCaptureService.class);
        service.putExtra(AudioCaptureService.EXTRA_RESULT_CODE, resultCode);
        service.putExtra(AudioCaptureService.EXTRA_DATA, data);
        if (Build.VERSION.SDK_INT >= 26) {
            activity.startForegroundService(service);
        } else {
            activity.startService(service);
        }
        return true;
    }

    public static String devicesJson() {
        JSONArray list = new JSONArray();
        try {
            AudioManager manager = manager();
            if (manager == null) {
                return list.toString();
            }
            int activeId = -1;
            if (Build.VERSION.SDK_INT >= 33) {
                List<AudioDeviceInfo> routed = manager.getAudioDevicesForAttributes(
                        new AudioAttributes.Builder().setUsage(AudioAttributes.USAGE_MEDIA).build());
                if (routed != null && !routed.isEmpty()) {
                    activeId = routed.get(0).getId();
                }
            }
            for (AudioDeviceInfo device : manager.getDevices(AudioManager.GET_DEVICES_OUTPUTS)) {
                if (!device.isSink()) {
                    continue;
                }
                JSONObject item = new JSONObject();
                item.put("id", String.valueOf(device.getId()));
                item.put("name", deviceName(device));
                item.put("channels", device.getChannelCounts().length == 0 ? 2 : device.getChannelCounts()[0]);
                boolean active = activeId >= 0
                        ? device.getId() == activeId
                        : device.getType() == AudioDeviceInfo.TYPE_BUILTIN_SPEAKER;
                item.put("default", active);
                item.put("active", active);
                list.put(item);
            }
        } catch (Exception ignored) {
        }
        return list.toString();
    }

    public static String sourcesJson() {
        JSONArray list = new JSONArray();
        try {
            AudioManager manager = manager();
            if (manager == null || Build.VERSION.SDK_INT < 26) {
                return list.toString();
            }
            String sessionApp = PlaybackNotificationListener.currentApp();
            for (AudioPlaybackConfiguration config : manager.getActivePlaybackConfigurations()) {
                JSONObject item = new JSONObject();
                String usage = usageLabel(config.getAudioAttributes().getUsage());
                item.put("pid", 0);
                item.put("name", sessionApp != null && !sessionApp.isEmpty() ? sessionApp : usage);
                item.put("icon", "");
                item.put("peak", 0);
                item.put("note", usage);
                list.put(item);
            }
        } catch (Exception ignored) {
        }
        return list.toString();
    }

    private static AudioManager manager() {
        Context context = activity != null ? activity.getApplicationContext() : null;
        if (context == null) {
            return null;
        }
        return (AudioManager) context.getSystemService(Context.AUDIO_SERVICE);
    }

    private static String deviceName(AudioDeviceInfo device) {
        CharSequence product = device.getProductName();
        String type = typeLabel(device.getType());
        if (product != null && product.length() > 0 && !product.toString().equals(android.os.Build.MODEL)) {
            return type + " — " + product;
        }
        return type;
    }

    private static String typeLabel(int type) {
        switch (type) {
            case AudioDeviceInfo.TYPE_BUILTIN_SPEAKER: return "Phone speaker";
            case AudioDeviceInfo.TYPE_WIRED_HEADPHONES: return "Wired headphones";
            case AudioDeviceInfo.TYPE_WIRED_HEADSET: return "Wired headset";
            case AudioDeviceInfo.TYPE_BLUETOOTH_A2DP: return "Bluetooth";
            case AudioDeviceInfo.TYPE_BLUETOOTH_SCO: return "Bluetooth call";
            case AudioDeviceInfo.TYPE_USB_HEADSET: return "USB headset";
            case AudioDeviceInfo.TYPE_HDMI: return "HDMI";
            case AudioDeviceInfo.TYPE_LINE_ANALOG: return "Line out";
            default: return "Output";
        }
    }

    private static String usageLabel(int usage) {
        switch (usage) {
            case AudioAttributes.USAGE_MEDIA: return "Media playback";
            case AudioAttributes.USAGE_GAME: return "Game";
            case AudioAttributes.USAGE_NOTIFICATION:
            case AudioAttributes.USAGE_NOTIFICATION_RINGTONE:
            case AudioAttributes.USAGE_NOTIFICATION_EVENT: return "Notification";
            case AudioAttributes.USAGE_ALARM: return "Alarm";
            case AudioAttributes.USAGE_ASSISTANCE_SONIFICATION: return "Interface sound";
            case AudioAttributes.USAGE_VOICE_COMMUNICATION: return "Call";
            case AudioAttributes.USAGE_ASSISTANT: return "Assistant";
            default: return "Playback";
        }
    }

    private static native void nativeRegister();
    private static native void nativeSetCaptureState(String state);
    static native void nativeSetLevels(float left, float right);
}
