"use strict";

globalThis.addEventListener("load", () => {
  console.log("IPUB SCRIPT LOADED");

  /** @type {Map<string, Element>} */
  const onScreenMap = new Map();

  const observer = new IntersectionObserver((entries) => {
    entries.forEach((e) => {
      if (e.intersectionRatio > 0) {
        console.debug(
          `IntersectionObserver: adding element #${e.target.id} to onScreenMap`,
        );
        onScreenMap.set(e.target.id, e.target);
      } else {
        console.debug(
          `IntersectionObserver: removing element #${e.target.id} to onScreenMap`,
        );
        onScreenMap.delete(e.target.id);
      }
    });
  });

  for (const element of document.querySelectorAll(
    `[data-ipub-trigger="on-screen"]`,
  )) {
    observer.observe(element);
  }

  document.addEventListener("scroll", async () => {
    for (const [id, element] of onScreenMap) {
      const perc = getPercentageInView(element);
      console.debug(`Element #${id} is now ${perc}% on screen`);

      const played = element.getAttribute("data-ipub-trigger-played") == "true";

      if (perc >= 100 && !played) {
        await playIpubElement(element);
        element.setAttribute("data-ipub-trigger-played", "true");
      }
    }
  });
});

/**
 * @param {Element} element
 */
async function playIpubElement(element) {
  switch (element.tagName) {
    case "audio": {
      /** @type {HTMLAudioElement} */
      const audio = element;

      await audio.play();

      break;
    }
    default:
      break;
  }
}

/**
 * @param {Element} element
 * @returns {number}
 */
function getPercentageInView(element) {
  const viewTop = globalThis.pageYOffset;
  const viewBottom = viewTop + globalThis.innerHeight;

  const rect = element.getBoundingClientRect();
  const elementTop = rect.y + viewTop;
  const elementBottom = rect.y + rect.height + viewTop;

  if (viewTop > elementBottom || viewBottom < elementTop) {
    return 0;
  }

  if (
    (viewTop < elementTop && viewBottom > elementBottom) ||
    (elementTop < viewTop && elementBottom > viewBottom)
  ) {
    return 100;
  }

  let inView = rect.height;

  if (elementTop < viewTop) {
    inView = rect.height - (globalThis.pageYOffset - elementTop);
  }

  if (elementBottom > viewBottom) {
    inView = inView - (elementBottom - viewBottom);
  }

  return Math.round((inView / globalThis.innerHeight) * 100);
}
