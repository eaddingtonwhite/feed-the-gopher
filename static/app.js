let username = null;
let input = null;
let scoreBoard = null
let socket = null;


function promptForUsername() {
    while (!!!username) {
        username = prompt("Enter your name: ");
    }
}

function initialize() {
    promptForUsername();
    document.querySelector("#name").innerHTML = username

    input = document.getElementById("chat-input");
    scoreBoard = document.getElementById("score-board");


    let connectHost = location.hostname + ":" + location.port;
    socket = new ReconnectingWebSocket(
        `ws://${connectHost}/connect`, null,
        {debug: true, reconnectInterval: 3000}
    );

    socket.onmessage = function (e) {
        let chat_messages = document.getElementById("chat-messages");
        chat_messages.insertAdjacentHTML(
            "beforeend",
            '<div class="message">' + e.data + '</div>'
        );
        chat_messages.scrollTop = chat_messages.scrollHeight;
    };

    input.addEventListener('keypress', e => {
        if (e.key === 'Enter') send()
    })
}

function send() {
    if (input.value === "") {
        return
    }
    fetch("/send-message", {
        method: "post",
        //make sure to serialize your JSON body
        body: JSON.stringify({
            Value: input.value,
            User: username,
        })
    })
    input.value = ""
}

function registerBtnHit() {
    fetch("/register-hit", {
        method: "post",
        body: JSON.stringify({
            user: username,
        })
    }).then(r => {
        if (r.status === 429) {
            alert("woah there partner slow down you can only feed so fast")
        }
    })
}

function buildAutoFeederBtnHit(type) {
    fetch("/build-auto-feeder", {
        method: "post",
        body: JSON.stringify({
            user: username,
            type: type

        })
    }).then(r => {
        if (r.status === 429) {
            alert("woah there partner slow down you can only feed so fast")
        }
    })
}

setInterval(() => {
    fetch("/top-scorers").then(async (resp) => {
        let scoreBoardContent = ""
        let scores = await resp.json()
        scores?.elements.forEach((v) => {
            scoreBoardContent += `<h3>${v.rank}: ${v.name} - ${v.value}</h3>`;
        })
        scoreBoard.innerHTML = scoreBoardContent
    })
}, 1000)

// Get all buy buttons
const buyButtons = document.querySelectorAll('.buy-button');

// Add click event listeners to each buy button
buyButtons.forEach(button => {
    button.addEventListener('click', () => {
        const cost = parseInt(button.getAttribute('data-cost'));

        // Get the corresponding "Number Owned" cell
        const numberOwnedCell = button.parentElement.nextElementSibling;

        // Get the current number owned value
        let numberOwned = parseInt(numberOwnedCell.textContent);


        numberOwned++;
        numberOwnedCell.textContent = numberOwned;
    });
});

window.onload = initialize
