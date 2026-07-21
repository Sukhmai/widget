//
//  ContactsImporter.swift
//  daily
//
//  Reads the address book so Social can hash the numbers and match friends.
//  Lives in the app target because widget extensions can't access Contacts.
//

import Contacts
import DailyKit

enum ContactsImporter {
    enum ImportError: Error { case accessDenied }

    /// Request Contacts access and return each contact's name + phone numbers.
    /// Throws `.accessDenied` if the user declines.
    static func fetch() async throws -> [Social.Contact] {
        let store = CNContactStore()

        let granted = try await store.requestAccess(for: .contacts)
        guard granted else { throw ImportError.accessDenied }

        let keys = [
            CNContactGivenNameKey,
            CNContactFamilyNameKey,
            CNContactPhoneNumbersKey,
            CNContactThumbnailImageDataKey,
        ] as [CNKeyDescriptor]

        // enumerateContacts is a synchronous, potentially slow full-book scan;
        // keep it off the main actor.
        return try await Task.detached(priority: .userInitiated) {
            var contacts: [Social.Contact] = []
            let request = CNContactFetchRequest(keysToFetch: keys)
            try store.enumerateContacts(with: request) { contact, _ in
                let numbers = contact.phoneNumbers.map { $0.value.stringValue }
                guard !numbers.isEmpty else { return }
                let name = "\(contact.givenName) \(contact.familyName)"
                    .trimmingCharacters(in: .whitespaces)
                contacts.append(Social.Contact(
                    name: name.isEmpty ? "Friend" : name,
                    numbers: numbers,
                    avatar: contact.thumbnailImageData
                ))
            }
            return contacts
        }.value
    }
}
