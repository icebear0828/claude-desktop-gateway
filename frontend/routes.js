export function mergeEditedRoute(routes, originalID, editedRoute) {
  const existing = routes.find((route) => route.desktopID === originalID || route.desktopID === editedRoute.desktopID);
  const nextRoute = {
    ...(existing || {}),
    ...editedRoute,
  };
  const nextRoutes = routes.filter((route) => route.desktopID !== originalID && route.desktopID !== editedRoute.desktopID);
  nextRoutes.push(nextRoute);
  nextRoutes.sort((left, right) => left.desktopID.localeCompare(right.desktopID));
  return nextRoutes;
}
