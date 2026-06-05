uni.addInterceptor({
  returnValue(res) {
    if (!(!!res && (typeof res === 'object' || typeof res === 'function') && typeof res.then === 'function')) {
      return res
    }
    return new Promise((resolve, reject) => {
      res.then((res) => res[1] ? reject(res[1]) : resolve(res[0]))
    })
  }
})
