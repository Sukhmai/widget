//
//  AppDelegate.swift
//  daily
//

import UIKit
import UserNotifications
import WidgetKit
import DailyKit
import os

private let log = Logger(subsystem: "skapur.sharedaily", category: "push")

/// Registers for silent background push so a friend's post can wake this app
/// and refresh the "story" widget immediately, instead of waiting on its own
/// poll interval. UIKit lifecycle is required here since SwiftUI's App
/// protocol has no remote-notification callbacks.
final class AppDelegate: NSObject, UIApplicationDelegate {
    func application(_ application: UIApplication,
                      didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]?) -> Bool {
        UNUserNotificationCenter.current().requestAuthorization(options: [.badge]) { granted, _ in
            log.info("notification authorization granted=\(granted)")
            guard granted else { return }
            DispatchQueue.main.async { application.registerForRemoteNotifications() }
        }
        return true
    }

    func application(_ application: UIApplication,
                      didRegisterForRemoteNotificationsWithDeviceToken deviceToken: Data) {
        let token = deviceToken.map { String(format: "%02x", $0) }.joined()
        log.info("registered for remote notifications, token=\(token)")
        Task { await Social.registerPushToken(token) }
    }

    func application(_ application: UIApplication,
                      didFailToRegisterForRemoteNotificationsWithError error: Error) {
        log.error("failed to register for remote notifications: \(error.localizedDescription)")
    }

    func application(_ application: UIApplication,
                      didReceiveRemoteNotification userInfo: [AnyHashable: Any],
                      fetchCompletionHandler completionHandler: @escaping (UIBackgroundFetchResult) -> Void) {
        log.info("received remote notification, reloading story timeline")
        WidgetCenter.shared.reloadTimelines(ofKind: "story")
        completionHandler(.newData)
    }
}
