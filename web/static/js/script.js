let db = null;

function initFirebase() {
    const config = {
        apiKey: "AIzaSyA1m4ffQkruhsPPXRPAwKFHYmrmIZGAxZI",
        authDomain: "kakuuchi-app.firebaseapp.com",
        projectId: "kakuuchi-app",
    };
    firebase.initializeApp(config);
}

function _login(app) {
    const provider = new firebase.auth.GoogleAuthProvider();
    provider.addScope('https://www.googleapis.com/auth/contacts.readonly');
    firebase.auth().signInWithPopup(provider).then(function (result) {
        app._user = result.user;
        app.user = { uid: result.user.uid };
        subscribeUser(app);
    }).catch(function (err) {
        console.log(err);
    });
}

function subscribeUser(app) {
    unsubscribeUser = db.doc("Users/" + app.user.uid).onSnapshot(user => {
        if (user.exists) {
            app.user = {
                displayName: user.data().displayName,
                photoURL: user.data().photoURL,
                uid: user.data().uid
            };

            app.muteUsers = user.data().muteUsers.filter(d => {
                return (new Date()) < d.expireTime;
            });
        } else {
            db.doc("Users/" + app.user.uid).set(
                {
                    displayName: app._user.displayName,
                    photoURL: app._user.photoURL,
                    uid: app._user.uid,
                    muteUsers: []
                }, {
                    merge: true
                });
        }
    });
}

function _logout(app) {
    unsubscribeUser();
    app.user = null;
    localStorage.removeItem("user");
    firebase.auth().signOut().catch(function (err) {
        console.log(err);
    });
}

function initApp(app, cb) {
    db = firebase.firestore();
    db.collection("Counters")
        .orderBy("lastUpdated", "desc")
        .onSnapshot(docs => {
            app.counters = [];
            let isExistSandbox = false;
            docs.forEach(doc => {
                let counter = doc.data();
                if (isShowCounter(app, counter)) {
                    counter.id = doc.id;
                    app.counters.push(counter);
                }
            });
            cb();
        })
}

function isShowCounter(app, counter) {
    let uid = (app.user == null) ? "" : app.user.uid;
    if (counter.owner.uid == uid) { return true; }
    if (counter.participant.indexOf(uid) > -1) { return true; }
    if (counter.isPrivate) { return false; }
    return true;
}

function _search(app) {
    if (app.searchWord == "") { return []; }
    axios.get("/search/?word=" + app.searchWord)
        .then(res => {
            app.searchedCounters = (res.data == null) ? [] : res.data;
        })
        .catch(err => {
            console.log(err);
        });

}

function _selectCounter(app) {
    app.messages = null;
    if (app.subscribeCounter != null) {
        app.unsubscribeCounter();
    }

    let counter = app.counters.filter(d => { return d.id == app.selectedCounterID; })[0];
    if (counter.isArchived) {
        axios.get(counter.link)
            .then(res => {
                app.messages = res.data.messages;
            })
            .catch(err => {
                console.log(err);
            });
    } else {
        app.unsubscribeCounter = db.collection("Counters/" + app.selectedCounterID + "/messages")
            .orderBy("inserted", "desc")
            .onSnapshot(messages => {
                app.messages = [];
                messages.forEach(message => {
                    app.messages.push(message.data());
                });
                if (app.isShowMessageInfo == null) {
                    app.isShowMessageInfo = true;
                } else if (app.isShowMessageInfo == true) {
                    app.isShowMessageInfo = false;
                };
            });
    }
    app.newMessage = "";
    location.hash = app.selectedCounterID;
}

function _submitMessage(app) {
    db.collection("Counters/" + app.selectedCounterID + "/messages").add({
        value: app.newMessage,
        user: app.user,
        inserted: new Date()
    }).then(doc => {
        touchLastUpdated(app);
        app.inSubmit = false;
        app.newMessage = "";
    }).catch(err => {
        console.log(err);
    });
}

function touchLastUpdated(app) {
    let participant = app.counters.filter(d => { return d.id == app.selectedCounterID; })[0].participant;
    if (participant.indexOf(app.user.uid) == -1) {
        participant.push(app.user.uid);
    }

    let props = {
        lastUpdated: new Date(),
        participant: participant,
        isIndexed: false
    };

    if (app.messages.length > app.messageCountLimit) {
        props.isArchived = true;
    }

    db.doc("Counters/" + app.selectedCounterID)
        .set(props, {
            merge: true
        });
}

function _addCounter(app) {
    let owner = app.user;
    delete owner.muteUsers;
    ref = db.collection("Counters").add({
        owner: owner,
        lastUpdated: new Date(),
        isPrivate: app.newCounterIsPrivate,
        name: app.newCounterName,
        description: app.newCounterDescription,
        isArchived: false,
        participant: [owner.uid],
        isIndexed: true,
        link: ""
    }).then(ref => {
        app.isOpenModal = false;
        app.newCounterIsPrivate = false;
        app.newCounterName = "";
        app.newCounterDescription = "";
    }).catch(err => {
        console.log(err);
    });
}

function updateUserInfo(app) {
    let user = app.user;
    if (user.uid != undefined) {
        localStorage.setItem("user", JSON.stringify(user));
        user.muteUsers = app.muteUsers || [];
        db.doc("Users/" + user.uid).set(user);
    }
}

function _formatMessage(message) {
    // http://www.din.or.jp/~ohzaki/perl.htm#httpURL
    let linkRegex = /(https?:\/\/[-_.!~*'()a-zA-Z0-9;\/?:@&=+$,%#]+)/g;
    message = message.replace(linkRegex, "<a href=\"$1\">$1</a>");
    return message;
}