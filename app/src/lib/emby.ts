export const openInEmby = (itemId: string, embyExternalUrl: string, serverId?: string) => {
  // Include context=home to match Emby UI routing expectations
  let embyUrl = `${embyExternalUrl}/web/index.html#!/item?id=${encodeURIComponent(itemId)}&context=home`;
  if (serverId) {
    embyUrl += `&serverId=${encodeURIComponent(serverId)}`;
  }
  window.open(embyUrl, '_blank', 'noopener,noreferrer');
};
