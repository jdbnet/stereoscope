package com.wails.app;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.Service;
import android.content.Intent;
import android.content.pm.ServiceInfo;
import android.media.AudioAttributes;
import android.media.AudioFormat;
import android.media.AudioManager;
import android.media.AudioPlaybackCaptureConfiguration;
import android.media.AudioRecord;
import android.media.projection.MediaProjection;
import android.media.projection.MediaProjectionManager;
import android.os.Build;
import android.os.Handler;
import android.os.IBinder;
import android.os.Looper;

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
        projection.registerCallback(new MediaProjection.Callback() {
            @Override
            public void onStop() {
                projection = null;
                releaseCapture();
                stopSelf();
            }
        }, new Handler(Looper.getMainLooper()));
        try {
            startRecorder();
        } catch (RuntimeException ex) {
            AudioBridge.nativeSetCaptureState("denied");
            releaseCapture();
            stopSelf();
        }
        return START_NOT_STICKY;
    }

    private void startRecorder() {
        AudioPlaybackCaptureConfiguration config = new AudioPlaybackCaptureConfiguration.Builder(projection)
                .addMatchingUsage(AudioAttributes.USAGE_MEDIA)
                .addMatchingUsage(AudioAttributes.USAGE_GAME)
                .addMatchingUsage(AudioAttributes.USAGE_UNKNOWN)
                .build();
        int sampleRate = outputSampleRate();
        // Positional stereo is what playback capture accepts. An index mask on
        // top of it makes some devices copy one channel onto both read slots.
        AudioFormat format = new AudioFormat.Builder()
                .setEncoding(AudioFormat.ENCODING_PCM_16BIT)
                .setSampleRate(sampleRate)
                .setChannelMask(AudioFormat.CHANNEL_IN_STEREO)
                .build();
        int min = AudioRecord.getMinBufferSize(sampleRate, AudioFormat.CHANNEL_IN_STEREO, AudioFormat.ENCODING_PCM_16BIT);
        if (min <= 0) {
            min = sampleRate * 4;
        }
        recorder = new AudioRecord.Builder()
                .setAudioPlaybackCaptureConfig(config)
                .setAudioFormat(format)
                .setBufferSizeInBytes(min * 2)
                .build();
        if (recorder.getState() != AudioRecord.STATE_INITIALIZED) {
            throw new IllegalStateException("playback capture was not initialised");
        }
        int channels = recorder.getChannelCount();
        if (channels < 2) {
            throw new IllegalStateException("playback capture did not open in stereo");
        }
        recorder.startRecording();
        running = true;
        AudioBridge.nativeSetCaptureState("capturing");
        final int frames = Math.max(sampleRate / 20, min / 4);
        Thread thread = new Thread(() -> {
            short[] samples = new short[frames * 2];
            int planarVotes = 0;
            int interleavedVotes = 0;
            boolean planar = false;
            while (running && recorder != null) {
                int n = recorder.read(samples, 0, samples.length);
                if (n < 4) {
                    continue;
                }
                if ((n & 1) == 1) {
                    n--;
                }
                int[] interleaved = channelPeaks(samples, n, false);
                int[] blocked = channelPeaks(samples, n, true);
                int interleavedGap = Math.abs(interleaved[0] - interleaved[1]);
                int blockedGap = Math.abs(blocked[0] - blocked[1]);
                if (blockedGap > interleavedGap + 2000) {
                    planarVotes++;
                } else if (interleavedGap > blockedGap + 2000) {
                    interleavedVotes++;
                }
                if (planarVotes > interleavedVotes + 2) {
                    planar = true;
                } else if (interleavedVotes > planarVotes + 2) {
                    planar = false;
                }
                int[] peaks = planar ? blocked : interleaved;
                AudioBridge.nativeSetLevels(peaks[0] / 32768f, peaks[1] / 32768f);
            }
        }, "stereoscope-capture");
        thread.start();
    }

    private static int[] channelPeaks(short[] samples, int count, boolean planar) {
        int left = 0;
        int right = 0;
        if (planar) {
            int half = count / 2;
            for (int i = 0; i < half; i++) {
                left = Math.max(left, Math.abs(samples[i]));
                right = Math.max(right, Math.abs(samples[half + i]));
            }
        } else {
            for (int i = 0; i + 1 < count; i += 2) {
                left = Math.max(left, Math.abs(samples[i]));
                right = Math.max(right, Math.abs(samples[i + 1]));
            }
        }
        return new int[] {left, right};
    }

    private int outputSampleRate() {
        AudioManager manager = (AudioManager) getSystemService(AUDIO_SERVICE);
        if (manager == null) {
            return SAMPLE_RATE;
        }
        String rate = manager.getProperty(AudioManager.PROPERTY_OUTPUT_SAMPLE_RATE);
        if (rate == null) {
            return SAMPLE_RATE;
        }
        try {
            int parsed = Integer.parseInt(rate);
            return parsed > 0 ? parsed : SAMPLE_RATE;
        } catch (NumberFormatException ignored) {
            return SAMPLE_RATE;
        }
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
    public void onTaskRemoved(Intent rootIntent) {
        releaseCapture();
        stopSelf();
        super.onTaskRemoved(rootIntent);
    }

    @Override
    public void onDestroy() {
        releaseCapture();
        super.onDestroy();
    }

    private void releaseCapture() {
        if (!running && recorder == null && projection == null) {
            return;
        }
        running = false;
        if (recorder != null) {
            try {
                recorder.stop();
            } catch (IllegalStateException ignored) {
            }
            recorder.release();
            recorder = null;
        }
        if (projection != null) {
            MediaProjection current = projection;
            projection = null;
            current.stop();
        }
        AudioBridge.nativeSetLevels(0, 0);
        AudioBridge.nativeSetCaptureState("idle");
    }

    @Override
    public IBinder onBind(Intent intent) {
        return null;
    }
}
