export const unreadEvent = 'pet-manager:messages-updated'

export function refreshUnreadCount() {
  window.dispatchEvent(new Event(unreadEvent))
}
