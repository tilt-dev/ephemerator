
// Add some light interactivity.
window.addEventListener('load', () => {
  let logpane = document.querySelector('.logpane')
  if (logpane) {
    // Scroll to bottom.
    logpane.scrollTop = logpane.scrollHeight
  }

  // Count down when the env will expire.
  let expirationEl = document.querySelector('.expiration')
  let countdownEl = document.querySelector('.expirationCountdown')
  let expiration = expirationEl && new Date(expirationEl.innerText)
  if (expiration && countdownEl && !isNaN(expiration.getTime())) {
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

function onRepoChange() {
  let repoSelect = document.querySelector('#repo')
  var params = new URLSearchParams(window.location.search)
  params.set("repo", repoSelect.value)
  window.location.search = params.toString()
}

function onBranchChange() {
  let repoSelect = document.querySelector('#repo')
  let branchSelect = document.querySelector('#branch')
  let pathSelect = document.querySelector('#path')
  var params = new URLSearchParams(window.location.search)
  params.set("repo", repoSelect.value)
  params.set("branch", branchSelect.value)
  params.set("path", pathSelect.value)
  window.location.search = params.toString()
}
