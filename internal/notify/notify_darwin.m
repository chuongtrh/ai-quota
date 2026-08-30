#import <Foundation/Foundation.h>
#import <UserNotifications/UserNotifications.h>

void aiquota_request_notification_permission(void) {
    @autoreleasepool {
        UNUserNotificationCenter *center = [UNUserNotificationCenter currentNotificationCenter];
        [center requestAuthorizationWithOptions:(UNAuthorizationOptionAlert | UNAuthorizationOptionSound)
                              completionHandler:^(BOOL granted, NSError *error) {
                                  (void)granted;
                                  (void)error;
                              }];
    }
}

int aiquota_send_notification(const char *title, const char *body) {
    if (title == NULL || body == NULL) {
        return 1;
    }
    @autoreleasepool {
        UNMutableNotificationContent *content = [[UNMutableNotificationContent alloc] init];
        content.title = [NSString stringWithUTF8String:title];
        content.body = [NSString stringWithUTF8String:body];
        content.sound = [UNNotificationSound defaultSound];

        NSString *identifier = [[NSUUID UUID] UUIDString];
        UNNotificationRequest *request = [UNNotificationRequest requestWithIdentifier:identifier
                                                                               content:content
                                                                               trigger:nil];
        [[UNUserNotificationCenter currentNotificationCenter]
            addNotificationRequest:request
            withCompletionHandler:^(NSError *error) {
                (void)error;
            }];
        [content release];
    }
    return 0;
}
