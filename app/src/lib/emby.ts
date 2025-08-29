export const openInEmby = (itemId: string, embyExternalUrl: string) => {
  const embyUrl = `${embyExternalUrl}/web/#!/item?id=${itemId}`;
  window.open(embyUrl, '_blank', 'noopener,noreferrer');
};
