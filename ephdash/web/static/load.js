
// Add some light interactivity.
window.addEventListener('load', () => {
  let logpane = document.querySelector('.logpane')
  logpane.scrollTop = logpane.scrollHeight

  let expirationEl = document.querySelector('.expiration')
  let countdownEl = document.querySelector('.expirationCountdown')
  let expiration = expirationEl && new Date(expirationEl.innerText)
  if (expiration && countdownEl) {
    let update = () => {
      let seconds = Math.ceil((expiration.getTime() - Date.now()) / 1000)
      if (seconds < 0) {
        countdownEl.innerHTML = `(Expired)`
      } else if (seconds > 120) {
        countdownEl.innerHTML = `(${Math.ceil(seconds/60)} minutes left)`
      } else {
        countdownEl.innerHTML = `(${seconds} seconds left)`
      }
    }
    update()
    setInterval(update, 1000)
  }
})
