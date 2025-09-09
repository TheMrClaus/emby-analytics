export const openInEmby = (itemId: string, embyExternalUrl: string, serverId?: string) => {
  let embyUrl = `${embyExternalUrl}/web/index.html#!/item?id=${itemId}`;
  if (serverId) {
    embyUrl += `&serverId=${serverId}`;
  }
  window.open(embyUrl, "_blank", "noopener,noreferrer");
};
