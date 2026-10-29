package com.wails.app;

import android.service.notification.NotificationListenerService;
import android.service.notification.StatusBarNotification;

/**
 * Reads the app name from a visible media notification when the user has
 * enabled notification access. Package names are not invented when this is off.
 */
public class PlaybackNotificationListener extends NotificationListenerService {
    private static volatile String appName = "";

    public static String currentApp() {
        return appName == null ? "" : appName;
    }

    @Override
    public void onNotificationPosted(StatusBarNotification notification) {
        remember(notification);
    }

    @Override
    public void onNotificationRemoved(StatusBarNotification notification) {
        if (notification != null && appName != null && appName.equals(label(notification))) {
            appName = "";
        }
    }

    private void remember(StatusBarNotification notification) {
        if (notification == null || notification.getNotification() == null) {
            return;
        }
        if (notification.getNotification().extras == null) {
            return;
        }
        String title = notification.getNotification().extras.getString("android.title");
        if (title == null || title.isEmpty()) {
            return;
        }
        String template = String.valueOf(notification.getNotification().extras.getString("android.template"));
        if (!template.contains("MediaStyle") && notification.getNotification().extras.getParcelable("android.mediaSession") == null) {
            return;
        }
        appName = label(notification);
    }

    private String label(StatusBarNotification notification) {
        try {
            CharSequence name = getPackageManager().getApplicationLabel(
                    getPackageManager().getApplicationInfo(notification.getPackageName(), 0));
            if (name != null && name.length() > 0) {
                return name.toString();
            }
        } catch (Exception ignored) {
        }
        return notification.getPackageName();
    }
}
