/* Barcelona Tapas Finder — client-side JS */

// ── Star rating widget ────────────────────────────────────────────────────────
(function () {
  const widget = document.getElementById("star-rating");
  if (!widget || !window.CURRENT_USER) return;

  const stars = widget.querySelectorAll(".star");
  const msg = document.getElementById("rating-msg");
  const rId = widget.dataset.restaurantId;

  function setHighlight(value) {
    stars.forEach((s, i) => s.classList.toggle("lit", i < value));
  }

  stars.forEach((star) => {
    star.addEventListener("mouseenter", () =>
      setHighlight(+star.dataset.value),
    );
    star.addEventListener("mouseleave", () => {
      const current = widget.dataset.current ? +widget.dataset.current : 0;
      setHighlight(current);
    });
    star.addEventListener("click", async () => {
      const value = +star.dataset.value;
      msg.textContent = "Saving…";
      try {
        const res = await fetch(`/api/restaurants/${rId}/ratings`, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            "user-id": window.CURRENT_USER.id,
          },
          body: JSON.stringify({ rating: value }),
        });
        if (!res.ok) throw new Error("Failed");
        const data = await res.json();
        widget.dataset.current = value;
        setHighlight(value);
        msg.textContent = `You rated this ${value} star${value !== 1 ? "s" : ""}. New average: ${Number(data.new_avg_rating).toFixed(1)}`;
      } catch {
        msg.textContent = "Could not save rating. Please try again.";
      }
    });
  });

  // Initialize highlight from server-rendered my_rating
  const initial = stars[0] ? +widget.querySelectorAll(".star.lit").length : 0;
  widget.dataset.current = initial;
})();
