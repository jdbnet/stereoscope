package com.wails.app;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.Service;
import android.content.Intent;
import android.content.pm.ServiceInfo;
import android.media.AudioAttributes;
import android.media.AudioFormat;
import android.media.AudioPlaybackCaptureConfiguration;
import android.media.AudioRecord;
import android.media.projection.MediaProjection;
import android.media.projection.MediaProjectionManager;
import android.os.Build;
import android.os.IBinder;

/**
 * Captures mix audio the user has consented to, and reports stereo peaks.
 * Apps that opt out of capture are absent from this mix.
 */
public class AudioCaptureService extends Service {
    public static final String EXTRA_RESULT_CODE = "resultCode";
    public static final String EXTRA_DATA = "data";
    private static final String CHANNEL = "stereoscope-capture";
    private static final int SAMPLE_RATE = 48000;

    private MediaProjection projection;
    private AudioRecord recorder;
    private volatile boolean running;

    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        if (Build.VERSION.SDK_INT < 29 || intent == null) {
            AudioBridge.nativeSetCaptureState("unsupported");
            stopSelf();
            return START_NOT_STICKY;
        }
        Notification notification = notification();
        if (Build.VERSION.SDK_INT >= 34) {
            startForeground(41, notification, ServiceInfo.FOREGROUND_SERVICE_TYPE_MEDIA_PROJECTION);
        } else {
            startForeground(41, notification);
        }
        int resultCode = intent.getIntExtra(EXTRA_RESULT_CODE, 0);
        Intent data = intent.getParcelableExtra(EXTRA_DATA);
        MediaProjectionManager manager = (MediaProjectionManager) getSystemService(MEDIA_PROJECTION_SERVICE);
        if (manager == null || data == null) {
            AudioBridge.nativeSetCaptureState("denied");
            stopSelf();
            return START_NOT_STICKY;
        }
        projection = manager.getMediaProjection(resultCode, data);
        if (projection == null) {
            AudioBridge.nativeSetCaptureState("denied");
            stopSelf();
            return START_NOT_STICKY;
        }
        startRecorder();
        return START_STICKY;
    }

    private void startRecorder() {
        AudioPlaybackCaptureConfiguration config = new AudioPlaybackCaptureConfiguration.Builder(projection)
                .addMatchingUsage(AudioAttributes.USAGE_MEDIA)
                .addMatchingUsage(AudioAttributes.USAGE_GAME)
                .addMatchingUsage(AudioAttributes.USAGE_UNKNOWN)
                .build();
        AudioFormat format = new AudioFormat.Builder()
                .setEncoding(AudioFormat.ENCODING_PCM_16BIT)
                .setSampleRate(SAMPLE_RATE)
                .setChannelMask(AudioFormat.CHANNEL_IN_STEREO)
                .build();
        int min = AudioRecord.getMinBufferSize(SAMPLE_RATE, AudioFormat.CHANNEL_IN_STEREO, AudioFormat.ENCODING_PCM_16BIT);
        if (min <= 0) {
            min = SAMPLE_RATE;
        }
        recorder = new AudioRecord.Builder()
                .setAudioPlaybackCaptureConfig(config)
                .setAudioFormat(format)
                .setBufferSizeInBytes(min * 2)
                .build();
        if (recorder.getState() != AudioRecord.STATE_INITIALIZED) {
            AudioBridge.nativeSetCaptureState("denied");
            stopSelf();
            return;
        }
        recorder.startRecording();
        running = true;
        AudioBridge.nativeSetCaptureState("capturing");
        final int buffer = min;
        Thread thread = new Thread(() -> {
            short[] samples = new short[buffer];
            while (running) {
                int n = recorder.read(samples, 0, samples.length);
                if (n <= 0) {
                    continue;
                }
                int left = 0;
                int right = 0;
                for (int i = 0; i + 1 < n; i += 2) {
                    left = Math.max(left, Math.abs(samples[i]));
                    right = Math.max(right, Math.abs(samples[i + 1]));
                }
                AudioBridge.nativeSetLevels(left / 32768f, right / 32768f);
            }
        }, "stereoscope-capture");
        thread.start();
    }

    private Notification notification() {
        NotificationManager manager = (NotificationManager) getSystemService(NOTIFICATION_SERVICE);
        if (Build.VERSION.SDK_INT >= 26 && manager != null) {
            NotificationChannel channel = new NotificationChannel(CHANNEL, "Audio monitoring", NotificationManager.IMPORTANCE_LOW);
            channel.setDescription("StereoScope is listening to playback you allowed it to capture.");
            manager.createNotificationChannel(channel);
        }
        Notification.Builder builder = Build.VERSION.SDK_INT >= 26
                ? new Notification.Builder(this, CHANNEL)
                : new Notification.Builder(this);
        return builder
                .setContentTitle("StereoScope")
                .setContentText("Monitoring playback on this device")
                .setSmallIcon(android.R.drawable.ic_media_play)
                .setOngoing(true)
                .build();
    }

    @Override
    public void onDestroy() {
        running = false;
        if (recorder != null) {
            try {
                recorder.stop();
            } catch (IllegalStateException ignored) {
            }
            recorder.release();
        }
        if (projection != null) {
            projection.stop();
        }
        AudioBridge.nativeSetLevels(0, 0);
        AudioBridge.nativeSetCaptureState("idle");
        super.onDestroy();
    }

    @Override
    public IBinder onBind(Intent intent) {
        return null;
    }
}
